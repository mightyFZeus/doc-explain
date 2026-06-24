package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey;not null" json:"id"`
	// WorkspaceID              uuid.UUID       `gorm:"type:uuid;not null;index" json:"workspaceId" validate:"required"`
	UserID                   uuid.UUID       `gorm:"type:uuid;not null;index" json:"userId" validate:"required"`
	Title                    string          `gorm:"type:varchar(255);not null" json:"title" validate:"required"`
	OriginalFilename         string          `gorm:"type:varchar(255);not null" json:"originalFilename" validate:"required"`
	FileType                 string          `gorm:"type:varchar(50);not null;index" json:"fileType" validate:"required"`
	SourceType               string          `gorm:"type:varchar(50);not null;index" json:"sourceType" validate:"required"`
	StorageKey               string          `gorm:"type:text;not null" json:"storageKey" validate:"required"`
	Status                   string          `gorm:"type:varchar(50);not null;default:'uploaded';index" json:"status"`
	Classification           string          `gorm:"type:varchar(100);index" json:"classification"`
	ClassificationConfidence float64         `gorm:"type:decimal(5,4)" json:"classificationConfidence"`
	Summary                  string          `gorm:"type:text" json:"summary"`
	PageCount                int             `gorm:"not null;default:0" json:"pageCount"`
	ChunkCount               int             `gorm:"not null;default:0" json:"chunkCount"`
	Version                  int             `gorm:"not null;default:1" json:"version"`
	Metadata                 json.RawMessage `gorm:"type:jsonb" json:"metadata"`
	CreatedAt                time.Time       `json:"createdAt"`
	UpdatedAt                time.Time       `json:"updatedAt"`
	DeletedAt                *time.Time      `gorm:"index" json:"deletedAt,omitempty"`
	ProccessingStatus        string          `gorm:"not null;default:'pending';index" json:"proccessingStatus"`
}

type CloudinaryUploadWebhookPayload struct {
	NotificationType    string              `json:"notification_type"`
	Timestamp           time.Time           `json:"timestamp"`
	RequestID           string              `json:"request_id"`
	AssetID             string              `json:"asset_id"`
	PublicID            string              `json:"public_id"`
	Version             int64               `json:"version"`
	VersionID           string              `json:"version_id"`
	Width               int                 `json:"width"`
	Height              int                 `json:"height"`
	Format              string              `json:"format"`
	ResourceType        string              `json:"resource_type"`
	CreatedAt           time.Time           `json:"created_at"`
	Tags                []string            `json:"tags"`
	Pages               int                 `json:"pages"`
	Bytes               int                 `json:"bytes"`
	Type                string              `json:"type"`
	Etag                string              `json:"etag"`
	Placeholder         bool                `json:"placeholder"`
	URL                 string              `json:"url"`
	SecureURL           string              `json:"secure_url"`
	AssetFolder         string              `json:"asset_folder"`
	DisplayName         string              `json:"display_name"`
	OriginalFilename    string              `json:"original_filename"`
	APIKey              string              `json:"api_key"`
	NotificationContext NotificationContext `json:"notification_context"`
	SignatureKey        string              `json:"signature_key"`
}

type NotificationContext struct {
	TriggeredAt time.Time   `json:"triggered_at"`
	TriggeredBy TriggeredBy `json:"triggered_by"`
}

type TriggeredBy struct {
	Source   string `json:"source"`
	ID       string `json:"id"`
	AuthType string `json:"auth_type"`
	AuthID   string `json:"auth_id"`
}

type Answer struct {
	Text    string         `json:"text"`
	Sources []AnswerSource `json:"sources"`
}

type AnswerSource struct {
	ChunkIndex int     `json:"chunkIndex"`
	Distance   float64 `json:"distance"`
}
