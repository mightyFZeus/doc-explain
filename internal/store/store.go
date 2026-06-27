package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"gorm.io/gorm"
)

var (
	ErrEmailAlreadyExists = errors.New("user already exists")
	ErrDocumentNotFound   = errors.New("document not found")
	ErrUserNotFound       = errors.New("user not found")
)

type Storage struct {
	Users interface {
		CreateUser(ctx context.Context, user models.User) error
		UserExists(ctx context.Context, email string) (bool, error)
		UserExistsByID(ctx context.Context, id uuid.UUID) (bool, error)
		LoginUser(ctx context.Context, email, password string) (*models.User, error)
	}
	Documents interface {
		SaveDocument(ctx context.Context, document models.Document) error
		GetAllDocuments(ctx context.Context, userID uuid.UUID) ([]models.Document, error)
		DocumentBelongsToUser(ctx context.Context, documentID uuid.UUID, userID uuid.UUID) (bool, error)
		DeleteDocument(ctx context.Context, documentID uuid.UUID, userID uuid.UUID) error
		EncryptPlaintextChunks(ctx context.Context, encrypt func(string) (string, error)) (int64, error)
		UpdateDocumentProcessingStatus(ctx context.Context, documentID uuid.UUID, status string, processingStatus string, chunkCount int) error
		ShouldProcessDocumentWebhook(ctx context.Context, documentID uuid.UUID) (bool, error)
		UpdateDocumentProcessingResult(ctx context.Context, documentID uuid.UUID, status string, processingStatus string, chunkCount int, classification string, confidence float64, summary string) error
		CreateChunks(ctx context.Context, chunks []models.DocumentChunk) error
		SearchDocumentChunks(
			ctx context.Context,
			documentID uuid.UUID,
			userID uuid.UUID,
			queryEmbedding []float64,
			limit int,
		) ([]models.RetrievedDocumentChunk, error)
	}
	Conversations interface {
		GetOrCreateByDocumentID(ctx context.Context, documentID uuid.UUID, userID uuid.UUID) (models.DocumentConversation, error)
		GetByDocumentID(ctx context.Context, documentID uuid.UUID, userID uuid.UUID) ([]models.DocumentConversation, error)
		CreateMessage(ctx context.Context, message models.DocumentMessage) error
		GetRecentMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]models.DocumentMessage, error)
	}
}

func NewStorage(db *gorm.DB) Storage {
	return Storage{
		Users:         &UserStore{db: db},
		Documents:     &DocumentStore{db: db},
		Conversations: &ConversationStore{db: db},
	}
}
