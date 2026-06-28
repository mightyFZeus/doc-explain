package jobs

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/internal/env"
)

const TypeProcessDocument = "document:process"

type ProcessDocumentPayload struct {
	DocumentID       string `json:"documentId"`
	AssetID          string `json:"assetId"`
	PublicID         string `json:"publicId"`
	SecureURL        string `json:"secureUrl"`
	ResourceType     string `json:"resourceType"`
	Format           string `json:"format"`
	OriginalFilename string `json:"originalFilename"`
	Bytes            int64  `json:"bytes"`
	Pages            int    `json:"pages"`
}

func NewProcessDocumentTask(payload ProcessDocumentPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	timeoutMinutes := env.GetInt("DOCUMENT_PROCESSING_TIMEOUT_MINUTES", 120)
	if timeoutMinutes <= 0 {
		timeoutMinutes = 120
	}

	return asynq.NewTask(
		TypeProcessDocument,
		body,
		asynq.Queue("rag"),
		asynq.MaxRetry(5),
		asynq.Timeout(time.Duration(timeoutMinutes)*time.Minute),
	), nil
}
