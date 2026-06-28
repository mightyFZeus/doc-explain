package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	EventUserSignup          = "user.signup"
	EventGuestSessionStarted = "guest.session_started"
	EventDocumentUploaded    = "document.uploaded"
	EventChunksCreated       = "document.chunks_created"
	EventDocumentClassified  = "document.classified"
	EventQuestionAsked       = "question.asked"
	EventAnswerGenerated     = "answer.generated"
)

type AnalyticsEvent struct {
	ID             uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	EventType      string          `gorm:"type:varchar(80);not null;index" json:"eventType"`
	ActorType      string          `gorm:"type:varchar(30);not null;index" json:"actorType"`
	UserID         *uuid.UUID      `gorm:"type:uuid;index" json:"userId,omitempty"`
	DocumentID     *uuid.UUID      `gorm:"type:uuid;index" json:"documentId,omitempty"`
	ConversationID *uuid.UUID      `gorm:"type:uuid;index" json:"conversationId,omitempty"`
	Count          int64           `gorm:"not null;default:1" json:"count"`
	DedupeKey      *string         `gorm:"type:varchar(255);uniqueIndex" json:"-"`
	Metadata       json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}
