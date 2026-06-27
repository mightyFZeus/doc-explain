package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/models"
)

type UserDto struct {
	FullName        string `gorm:"not null" json:"fullName" validate:"required,min=2,max=255"`
	Email           string `gorm:"not null" json:"email" validate:"required,min=2,max=255"`
	Password        string `gorm:"not null" json:"password" validate:"required,min=2,max=72"`
	ConfirmPassword string `gorm:"not null" json:"confirmPassword" validate:"required"`
	TermsAccepted   bool   `gorm:"type:boolean;not null" json:"termsAccepted" validate:"required,eq=true"`
}

type SearchDocumentDto struct {
	Query      string    ` json:"query" validate:"required"`
	DocumentID uuid.UUID ` json:"documentId" validate:"required"`
}

type DocumentStatusEvent struct {
	DocumentID       string    `json:"documentId"`
	Status           string    `json:"status"`           // processing, ready, failed
	ProcessingStatus string    `json:"processingStatus"` // processing, completed, failed
	ChunkCount       int       `json:"chunkCount"`
	Error            string    `json:"error,omitempty"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

const DocumentStatusChannel = "document_status_events"

type LoginDto struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	User  *models.User `json:"user"`
	Token string       `json:"token"`
}
