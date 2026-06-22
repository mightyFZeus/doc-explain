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
	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/models"
)

var allowedTypes = map[string]bool{
	"application/pdf": true,
	"image/jpeg":      true,
	"image/png":       true,
	"application/zip": false,
}

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
	_, err = file.Read(buffer)
	if err != nil {
		http.Error(w, "Cannot read file safely", http.StatusInternalServerError)
		return
	}

	_, _ = file.Seek(0, 0)

	detectedType := http.DetectContentType(buffer)

	if !allowedTypes[detectedType] {
		http.Error(w, "File type not allowed", http.StatusUnsupportedMediaType)
		return
	}

	fileName := strings.ToLower(fileHeader.Filename)
	if !strings.HasSuffix(fileName, ".pdf") &&
		!strings.HasSuffix(fileName, ".docx") &&
		!strings.HasSuffix(fileName, ".png") &&
		!strings.HasSuffix(fileName, ".jpg") {
		http.Error(w, "File extension mismatch", http.StatusUnsupportedMediaType)
		return
	}

	// upload to cloudinary
	uploadResult, err := app.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder:       "documents",
		ResourceType: "auto",
	})
	if err != nil {
		app.logger.Errorf("error", "upload to cloudibary failed", err)
		app.internalServerError(w, r, errors.New("Failed to upload file"))
		return
	}
	app.jsonResponse(w, http.StatusOK, map[string]string{
		"url":    uploadResult.URL,
		"status": "success",
	})

}

func (app *application) CloudinaryUploadWebhook(w http.ResponseWriter, r *http.Request) {

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

	// create task payload
	taskPayload := jobs.ProcessDocumentPayload{
		DocumentID: payload.PublicID,
		AssetID:    payload.AssetID,
		PublicID:   payload.PublicID,
		SecureURL:  payload.SecureURL,
		Format:     payload.Format,
		Bytes:      int64(payload.Bytes),
		Pages:      payload.Pages,
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
