package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/models"
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

	isValid := verifyCloudinaryWebhook(bodyBytes, timestamp, signature, apiSecret)
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

	documentID, err := documentIDFromPublicID(payload.PublicID)
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

func verifyCloudinaryWebhook(body []byte, timestamp int64, signature, apiSecret string) bool {
	var minifiedBuffer bytes.Buffer
	if err := json.Compact(&minifiedBuffer, body); err != nil {
		return false
	}

	payloadStr := minifiedBuffer.String() + strconv.FormatInt(timestamp, 10) + apiSecret

	hash := sha1.Sum([]byte(payloadStr))
	expectedSignature := hex.EncodeToString(hash[:])

	return subtle.ConstantTimeCompare(
		[]byte(expectedSignature),
		[]byte(signature),
	) == 1
}

func documentIDFromPublicID(publicID string) (uuid.UUID, error) {
	value := strings.TrimSpace(publicID)
	if index := strings.LastIndex(value, "/"); index >= 0 {
		value = value[index+1:]
	}

	documentID, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errors.New("invalid cloudinary public_id")
	}

	return documentID, nil
}
