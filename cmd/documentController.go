package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/internal/dtos"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"github.com/openai/openai-go/v2"
)

func (app *application) UploadDocumentHandler(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	// check size
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		app.badRequestResponse(w, r, errors.New("File too large"))
		return
	}

	// get file from the upload
	file, fileHeader, err := r.FormFile("document")
	if err != nil {
		app.badRequestResponse(w, r, errors.New("'document'  is required"))
		return
	}
	defer file.Close()

	// read the content type of the first 512 bytes
	buffer := make([]byte, 512)
	bytesRead, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		app.logger.Errorw("read file failed", "error", err)
		app.internalServerError(w, r, errors.New("Cannot read file safely"))
		return
	}

	_, _ = file.Seek(0, 0)

	fileName := strings.ToLower(fileHeader.Filename)
	detectedType := http.DetectContentType(buffer[:bytesRead])
	if !IsAllowedDocumentUpload(fileName, detectedType) {
		http.Error(w, "File type not allowed", http.StatusUnsupportedMediaType)
		return
	}

	userID := uuid.Nil
	if value := strings.TrimSpace(r.FormValue("userId")); value != "" {
		parsedUserID, err := uuid.Parse(value)
		if err != nil {
			app.badRequestResponse(w, r, errors.New("invalid userId"))
			return
		}
		userID = parsedUserID
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = fileHeader.Filename
	}

	documentID := uuid.New()

	// upload to cloudinary
	uploadResult, err := app.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder:       "documents",
		ResourceType: "auto",
		PublicID:     documentID.String(),
	})
	if err != nil {
		app.logger.Errorw("upload to cloudinary failed", "error", err)
		app.internalServerError(w, r, errors.New("Failed to upload file"))
		return
	}

	metadata, err := json.Marshal(map[string]any{
		"asset_id":           uploadResult.AssetID,
		"public_id":          uploadResult.PublicID,
		"secure_url":         uploadResult.SecureURL,
		"url":                uploadResult.URL,
		"resource_type":      uploadResult.ResourceType,
		"format":             uploadResult.Format,
		"bytes":              uploadResult.Bytes,
		"cloudinary_version": uploadResult.Version,
	})
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	document := models.Document{
		ID:                documentID,
		UserID:            userID,
		Title:             title,
		OriginalFilename:  fileHeader.Filename,
		FileType:          detectedType,
		SourceType:        "upload",
		StorageKey:        uploadResult.PublicID,
		Status:            "uploaded",
		PageCount:         uploadResult.Pages,
		Version:           1,
		Metadata:          json.RawMessage(metadata),
		ProccessingStatus: "uploaded",
	}
	if err := app.store.Documents.SaveDocument(ctx, document); err != nil {
		app.internalServerError(w, r, err)
		return
	}

	app.jsonResponse(w, http.StatusOK, map[string]string{
		"documentId": documentID.String(),
		"url":        uploadResult.URL,
		"status":     "success",
	})

}

func (app *application) CloudinaryUploadWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	signature := r.Header.Get("X-Cld-Signature")
	timestampStr := r.Header.Get("X-Cld-Timestamp")

	if signature == "" || timestampStr == "" {
		http.Error(w, "Missing security headers", http.StatusUnauthorized)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid timestamp header", http.StatusUnauthorized)
		return
	}

	// checkingif the webhook is recent, < 5 mins
	currentTime := time.Now().Unix()
	if currentTime-timestamp > 300 {
		app.unauthorizedResponse(w, r, errors.New("Webhook request expired"))
		return
	}

	apiSecret := env.GetString("CLOUDINARY_API_SECRET", "")

	isValid := VerifyCloudinaryWebhook(bodyBytes, timestamp, signature, apiSecret)
	if !isValid {
		app.logger.Errorf("error", "Invalid signature. Request source unverified.")
		app.unauthorizedResponse(w, r, errors.New("Invalid signature."))
		return
	}

	var payload models.CloudinaryUploadWebhookPayload
	if err := readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	documentID, err := DocumentIDFromPublicID(payload.PublicID)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	shouldProcess, err := app.store.Documents.ShouldProcessDocumentWebhook(ctx, documentID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}
	if !shouldProcess {
		app.jsonResponse(w, http.StatusOK, map[string]string{
			"status": "already_queued",
		})
		return
	}

	// create task payload
	taskPayload := jobs.ProcessDocumentPayload{

		DocumentID:       documentID.String(),
		AssetID:          payload.AssetID,
		PublicID:         payload.PublicID,
		SecureURL:        payload.SecureURL,
		Format:           payload.Format,
		OriginalFilename: payload.OriginalFilename,
		Bytes:            int64(payload.Bytes),
		Pages:            payload.Pages,
	}

	// create task
	tasks, err := jobs.NewProcessDocumentTask(taskPayload)
	if err != nil {
		app.logger.Errorf("error", "Failed to create task", err)
		app.internalServerError(w, r, err)
		return
	}

	// queue task
	info, err := app.asynqClient.Enqueue(
		tasks,
		asynq.Queue("rag"),
		asynq.Unique(24*time.Hour),
		asynq.TaskID("document:process:"+documentID.String()),
	)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	app.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "queued",
		"jobId":  info.ID,
	})

}

func (app *application) SearchThroughDocumentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload dtos.SearchDocumentDto
	if err := app.DecodeAndValidate(w, r, &payload); err != nil {
		return
	}
	query := strings.TrimSpace(payload.Query)

	queryEmbedding, err := app.service.Embedder.Embed(ctx, query)
	if err != nil {
		app.logger.Errorw("error", "Failed to embed query", err)
		app.internalServerError(w, r, err)
		return
	}
	chunks, err := app.store.Documents.SearchDocumentChunks(
		ctx,
		payload.DocumentID,
		queryEmbedding,
		app.service.TopK,
	)
	if err != nil {
		app.logger.Errorw("error", "Failed to search document chunks", err)
		app.internalServerError(w, r, err)
		return
	}
	if len(chunks) == 0 {
		app.jsonResponse(w, http.StatusOK, map[string]string{
			"status": "no_results",
		})
		return
	}
	contextText := FormatChunksForPrompt(chunks)

	// Set headers  streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	stream := app.service.OpenAI.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(app.service.LLMModel),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(fmt.Sprintf(`
You are a strict, factual assistant that answers questions using ONLY the provided document excerpts. 

CRITICAL RULE: Before generating a single word of output, perform an internal check. Does the provided text contain explicit answers to EVERY parameter requested in the prompt without needing unauthorized calculations or stitching completely unrelated contexts together? If NO, your entire output must strictly be "I could not find that in this document." and nothing else. Do not start a sentence explaining partial facts.

Rules:
1. Context Isolation: Treat separate sections, companies, or projects in the text as isolated. Do NOT link a skill, technology, or action from one section to an event or project in another unless the text explicitly combines them. 
   - EXCEPTION: You may cross-reference facts from separate chunks ONLY if the user's question explicitly names both sections/topics (e.g., "under Section 13 and Section 19").

2. Strict Truthfulness: If the context does not contain the answer, or if the question implicitly links two unrelated facts from the text, you must say: "I could not find that in this document."

3. No Hallucinations: Do not assume, extrapolate, or use outside knowledge. If the exact relationship asked about is not written down, it does not exist.

4. Formatting: Be highly concise. Cite chunk numbers like [chunk 3] at the end of relevant sentences.

5. No Mathematical Extrapolation: Do NOT perform arithmetic calculations (addition, subtraction, division, averages) on numbers found in the text to derive new statistics or financials. 
   - EXCEPTION: You are permitted to map logical time equivalents based on standard language (e.g., understanding that "two quarters" equals "half-yearly" or "six months"), but you must never compute financial balances or invent unwritten statistics.

6. Multi-Part Query Compliance: If a question asks for a specific combination of answers (e.g., a validity statement AND a service method), you must locate BOTH components in the retrieved text. If either component is completely missing or unwritten, do not attempt a partial answer. Fall back instantly to: "I could not find that in this document."


Question:
%s

Document excerpts:
%s

Answer:
`, query, contextText)),
		},
		MaxTokens: openai.Int(1200),
	})

	// Stream the response
	for stream.Next() {
		event := stream.Current()
		if len(event.Choices) > 0 && event.Choices[0].Delta.Content != "" {
			chunkData, _ := json.Marshal(map[string]string{"content": event.Choices[0].Delta.Content})
			fmt.Fprintf(w, "data: %s\n\n", chunkData)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}

	if err := stream.Err(); err != nil {
		app.logger.Errorw("error", "Streaming failed", err)
		errorData, _ := json.Marshal(map[string]string{"error": "Failed to generate answer"})
		fmt.Fprintf(w, "data: %s\n\n", errorData)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Send done signal
	fmt.Fprintf(w, "data: {\"done\": true}\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
