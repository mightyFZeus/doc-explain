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
	"github.com/gorilla/websocket"
	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/cmd/helpers"
	"github.com/mightyfzeus/doc-explain/internal/dtos"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"github.com/mightyfzeus/doc-explain/internal/store"
	"github.com/openai/openai-go/v2"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (app *application) authenticatedUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	user, err := helpers.GetUserFromContext(r.Context())
	if err != nil {
		app.logger.Errorw("can't get user from context", "error", err)
		app.unauthorizedResponse(w, r, err)
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(user.UserID)
	if err != nil {
		app.logger.Errorw("can't parse user id", "error", err)
		app.unauthorizedResponse(w, r, errors.New("invalid user id"))
		return uuid.Nil, false
	}

	return userID, true
}

func (app *application) UploadDocumentHandler(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	userID, ok := app.authenticatedUserID(w, r)
	if !ok {
		return
	}

	// check size
	err2 := r.ParseMultipartForm(10 << 20)
	if err2 != nil {
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
	if !helpers.IsAllowedDocumentUpload(fileName, detectedType) {
		http.Error(w, "File type not allowed", http.StatusUnsupportedMediaType)
		return
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

func (app *application) GetAllDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := app.authenticatedUserID(w, r)
	if !ok {
		return
	}

	documents, err := app.store.Documents.GetAllDocuments(ctx, userID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	app.jsonResponse(w, http.StatusOK, map[string]any{
		"documents": documents,
		"count":     len(documents),
	})
}

func (app *application) DeleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := app.authenticatedUserID(w, r)
	if !ok {
		return
	}

	documentIdStr := r.URL.Query().Get("documentId")
	if documentIdStr == "" {
		app.badRequestResponse(w, r, errors.New("documentId is required"))
		return
	}

	documentID, err := uuid.Parse(documentIdStr)
	if err != nil {
		app.badRequestResponse(w, r, errors.New("invalid documentId"))
		return
	}

	if err := app.store.Documents.DeleteDocument(ctx, documentID, userID); err != nil {
		if errors.Is(err, store.ErrDocumentNotFound) {
			app.notFoundResponse(w, r, err)
			return
		}
		app.internalServerError(w, r, err)
		return
	}

	app.jsonResponse(w, http.StatusOK, map[string]string{
		"documentId": documentID.String(),
		"status":     "deleted",
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

	isValid := helpers.VerifyCloudinaryWebhook(bodyBytes, timestamp, signature, apiSecret)
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

	documentID, err := helpers.DocumentIDFromPublicID(payload.PublicID)
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

	userID, ok := app.authenticatedUserID(w, r)
	if !ok {
		return
	}

	var payload dtos.SearchDocumentDto
	if err := helpers.DecodeAndValidate(w, r, &payload); err != nil {
		return
	}
	query := strings.TrimSpace(payload.Query)

	conversation, err := app.store.Conversations.GetOrCreateByDocumentID(ctx, payload.DocumentID, userID)
	if err != nil {
		if errors.Is(err, store.ErrDocumentNotFound) {
			app.notFoundResponse(w, r, err)
			return
		}
		app.internalServerError(w, r, err)
		return
	}

	history, err := app.store.Conversations.GetRecentMessages(ctx, conversation.ID, 6)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	standaloneQuery, err := helpers.RewriteQueryForRetrieval(ctx, query, history)
	if err != nil {
		app.logger.Errorf("error", "Failed to rewrite query", err)
		app.internalServerError(w, r, err)
		return
	}
	if err := app.store.Conversations.CreateMessage(ctx, models.DocumentMessage{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        query,
	}); err != nil {
		app.logger.Errorf("error", "Failed to create message", err)
		app.internalServerError(w, r, err)
		return
	}

	queryEmbedding, err := app.service.Embedder.Embed(ctx, standaloneQuery)
	if err != nil {
		app.logger.Errorw("error", "Failed to embed query", err)
		app.internalServerError(w, r, err)
		return
	}
	chunks, err := app.store.Documents.SearchDocumentChunks(
		ctx,
		payload.DocumentID,
		userID,
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
	for i := range chunks {
		chunks[i].Content, err = app.service.ChunkCipher.Decrypt(chunks[i].Content)
		if err != nil {
			app.logger.Errorw("error", "Failed to decrypt chunk content", err)
			app.internalServerError(w, r, err)
			return
		}
	}
	contextText := helpers.FormatChunksForPrompt(chunks)
	prompt := helpers.GetPrompt(history, contextText, query)

	// Set headers  streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	stream := app.service.OpenAI.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(app.service.LLMModel),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		MaxTokens: openai.Int(1200),
	})

	var answer strings.Builder
	// Stream the response
	for stream.Next() {
		event := stream.Current()
		if len(event.Choices) > 0 && event.Choices[0].Delta.Content != "" {
			content := event.Choices[0].Delta.Content
			answer.WriteString(content)
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
	if err := app.store.Conversations.CreateMessage(ctx, models.DocumentMessage{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        answer.String(),
	}); err != nil {
		app.logger.Errorw("failed to save assistant message", "error", err)
	}

	// Send done signal
	fmt.Fprintf(w, "data: {\"done\": true}\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (app *application) GetDocumentConversationsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := app.authenticatedUserID(w, r)
	if !ok {
		return
	}

	documentIDParam := strings.TrimSpace(r.URL.Query().Get("documentId"))

	if documentIDParam == "" {
		app.badRequestResponse(w, r, errors.New("documentId is required"))
		return
	}

	documentID, err := uuid.Parse(documentIDParam)
	if err != nil {
		app.badRequestResponse(w, r, errors.New("invalid documentId"))
		return
	}

	conversations, err := app.store.Conversations.GetByDocumentID(ctx, documentID, userID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	app.jsonResponse(w, http.StatusOK, map[string]any{
		"documentId":        documentID.String(),
		"conversations":     conversations,
		"conversationCount": len(conversations),
	})
}

func (app *application) DocumentStatusWSHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.authenticatedUserID(w, r)
	if !ok {
		return
	}

	documentIDParam := r.URL.Query().Get("documentId")
	if documentIDParam == "" {
		app.badRequestResponse(w, r, errors.New("documentId is required"))
		return
	}

	documentID, err := uuid.Parse(documentIDParam)
	if err != nil {
		app.badRequestResponse(w, r, errors.New("invalid documentId"))
		return
	}

	belongsToUser, err := app.store.Documents.DocumentBelongsToUser(r.Context(), documentID, userID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}
	if !belongsToUser {
		app.notFoundResponse(w, r, store.ErrDocumentNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		app.logger.Errorw("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	app.docHub.Add(documentID.String(), conn)
	defer app.docHub.Delete(documentID.String(), conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
