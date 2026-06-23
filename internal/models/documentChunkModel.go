package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Vector []float64

func (v Vector) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}

	values := make([]string, len(v))
	for i, value := range v {
		values[i] = strconv.FormatFloat(value, 'f', -1, 64)
	}

	return "[" + strings.Join(values, ",") + "]", nil
}

func (v *Vector) Scan(src any) error {
	if src == nil {
		*v = nil
		return nil
	}

	var value string
	switch typed := src.(type) {
	case string:
		value = typed
	case []byte:
		value = string(typed)
	default:
		return fmt.Errorf("cannot scan vector from %T", src)
	}

	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if value == "" {
		*v = Vector{}
		return nil
	}

	parts := strings.Split(value, ",")
	vector := make(Vector, len(parts))
	for i, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return err
		}
		vector[i] = number
	}

	*v = vector
	return nil
}

type DocumentChunk struct {
	ID             uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey;not null" json:"id"`
	DocumentID     uuid.UUID       `gorm:"type:uuid;not null;index;uniqueIndex:idx_document_chunks_document_chunk" json:"documentId"`
	ChunkIndex     int             `gorm:"not null;uniqueIndex:idx_document_chunks_document_chunk" json:"chunkIndex"`
	Content        string          `gorm:"type:text;not null" json:"content"`
	TokenSize      int             `gorm:"not null;default:0" json:"tokenSize"`
	StartSentence  int             `gorm:"not null;default:0" json:"startSentence"`
	EndSentence    int             `gorm:"not null;default:0" json:"endSentence"`
	Embedding      Vector          `gorm:"type:vector(1536);not null" json:"embedding"`
	EmbeddingModel string          `gorm:"type:varchar(100);not null" json:"embeddingModel"`
	EmbeddingDim   int             `gorm:"not null;default:1536" json:"embeddingDim"`
	Metadata       json.RawMessage `gorm:"type:jsonb" json:"metadata"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}
