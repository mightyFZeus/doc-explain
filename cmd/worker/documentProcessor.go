package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/store"
	"github.com/teilomillet/raggo"
	"go.uber.org/zap"
)

type DocumentProcessor struct {
	// Add dependencies here later:
	store      store.Storage
	logger     *zap.SugaredLogger
	cloudinary *cloudinary.Cloudinary
	raggo      *raggo.SimpleRAG
}

func NewDocumentProcessor() *DocumentProcessor {
	return &DocumentProcessor{}
}

func (p *DocumentProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload jobs.ProcessDocumentPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("invalid process document payload: %w: %v", asynq.SkipRetry, err)
	}

	if payload.DocumentID == "" {
		return fmt.Errorf("documentId is required: %w", asynq.SkipRetry)
	}

	if payload.AssetID == "" {
		return fmt.Errorf("assetId is required: %w", asynq.SkipRetry)
	}

	if payload.PublicID == "" {
		return fmt.Errorf("publicId is required: %w", asynq.SkipRetry)
	}

	if payload.SecureURL == "" {
		return fmt.Errorf("secureUrl is required: %w", asynq.SkipRetry)
	}

	// TODO: Get the document from cloudinary

	// TODO: handle raggo chunk

	// TODO: Store chunks + embeddings in DB

	// TODO: Mark document as ready in the DB

	return nil
}
