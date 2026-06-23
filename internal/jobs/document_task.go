package jobs

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const TypeProcessDocument = "document:process"

type ProcessDocumentPayload struct {
	DocumentID       string `json:"documentId"`
	AssetID          string `json:"assetId"`
	PublicID         string `json:"publicId"`
	SecureURL        string `json:"secureUrl"`
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

	return asynq.NewTask(
		TypeProcessDocument,
		body,
		asynq.Queue("rag"),
		asynq.MaxRetry(5),
		asynq.Timeout(30*time.Minute),
	), nil
}
