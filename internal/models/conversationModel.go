package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DocumentConversation struct {
	ID         uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	DocumentID uuid.UUID         `gorm:"type:uuid;not null;index" json:"documentId"`
	UserID     uuid.UUID         `gorm:"type:uuid;index" json:"userId"`
	Title      string            `gorm:"type:varchar(255)" json:"title"`
	Summary    string            `gorm:"type:text" json:"summary"`
	Messages   []DocumentMessage `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

type DocumentMessage struct {
	ID             uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ConversationID uuid.UUID       `gorm:"type:uuid;not null;index" json:"conversationId"`
	Role           string          `gorm:"type:varchar(20);not null" json:"role"`
	Content        string          `gorm:"type:text;not null" json:"content"`
	Metadata       json.RawMessage `gorm:"type:jsonb" json:"metadata"`
	CreatedAt      time.Time       `json:"createdAt"`
}
