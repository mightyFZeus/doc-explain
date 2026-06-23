package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"gorm.io/gorm"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type Storage struct {
	Users interface {
		CreateUser(ctx context.Context, user models.User) error
		UserExists(ctx context.Context, email string) (bool, error)
		UserExistsByID(ctx context.Context, id uuid.UUID) (bool, error)
	}
	Documents interface {
		SaveDocument(ctx context.Context, document models.Document) error
		UpdateDocumentProcessingStatus(ctx context.Context, documentID uuid.UUID, status string, processingStatus string, chunkCount int) error
		ShouldProcessDocumentWebhook(ctx context.Context, documentID uuid.UUID) (bool, error)
		UpdateDocumentProcessingResult(ctx context.Context, documentID uuid.UUID, status string, processingStatus string, chunkCount int, classification string, confidence float64, summary string) error
		CreateChunks(ctx context.Context, chunks []models.DocumentChunk) error
	}
}

func NewStorage(db *gorm.DB) Storage {
	return Storage{
		Users:     &UserStore{db: db},
		Documents: &DocumentStore{db: db},
	}
}
