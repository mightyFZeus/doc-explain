package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DocumentStore struct {
	db *gorm.DB
}

func (ds *DocumentStore) SaveDocument(ctx context.Context, document models.Document) error {
	return ds.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id",
				"title",
				"original_filename",
				"file_type",
				"source_type",
				"storage_key",
				"status",
				"classification",
				"classification_confidence",
				"summary",
				"page_count",
				"chunk_count",
				"version",
				"metadata",
				"proccessing_status",
				"updated_at",
			}),
		}).
		Create(&document).Error
}

func (ds *DocumentStore) GetAllDocuments(ctx context.Context) ([]models.Document, error) {
	var documents []models.Document

	err := ds.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Find(&documents).Error

	return documents, err
}

func (ds *DocumentStore) DeleteDocument(ctx context.Context, documentID uuid.UUID) error {
	return ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.
			Unscoped().
			Model(&models.Document{}).
			Where("id = ?", documentID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrDocumentNotFound
		}

		var conversationIDs []uuid.UUID
		if err := tx.
			Model(&models.DocumentConversation{}).
			Where("document_id = ?", documentID).
			Pluck("id", &conversationIDs).Error; err != nil {
			return err
		}

		if len(conversationIDs) > 0 {
			if err := tx.
				Where("conversation_id IN ?", conversationIDs).
				Delete(&models.DocumentMessage{}).Error; err != nil {
				return err
			}
		}

		if err := tx.
			Where("document_id = ?", documentID).
			Delete(&models.DocumentConversation{}).Error; err != nil {
			return err
		}

		if err := tx.
			Where("document_id = ?", documentID).
			Delete(&models.DocumentChunk{}).Error; err != nil {
			return err
		}

		return tx.
			Unscoped().
			Where("id = ?", documentID).
			Delete(&models.Document{}).Error
	})
}

func (ds *DocumentStore) EncryptPlaintextChunks(ctx context.Context, encrypt func(string) (string, error)) (int64, error) {
	var encryptedCount int64

	err := ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var chunks []struct {
			ID      uuid.UUID
			Content string
		}

		if err := tx.
			Model(&models.DocumentChunk{}).
			Select("id", "content").
			Where("content NOT LIKE ?", "v1:%").
			Find(&chunks).Error; err != nil {
			return err
		}

		for _, chunk := range chunks {
			encryptedContent, err := encrypt(chunk.Content)
			if err != nil {
				return err
			}

			if err := tx.
				Model(&models.DocumentChunk{}).
				Where("id = ?", chunk.ID).
				Update("content", encryptedContent).Error; err != nil {
				return err
			}

			encryptedCount++
		}

		return nil
	})

	return encryptedCount, err
}

func (ds *DocumentStore) UpdateDocumentProcessingStatus(ctx context.Context, documentID uuid.UUID, status string, processingStatus string, chunkCount int) error {
	updates := map[string]any{
		"status":             status,
		"proccessing_status": processingStatus,
		"updated_at":         gorm.Expr("NOW()"),
	}

	if chunkCount > 0 {
		updates["chunk_count"] = chunkCount
	}

	return ds.db.WithContext(ctx).
		Model(&models.Document{}).
		Where("id = ?", documentID).
		Updates(updates).Error
}

func (ds *DocumentStore) ShouldProcessDocumentWebhook(ctx context.Context, documentID uuid.UUID) (bool, error) {
	result := ds.db.WithContext(ctx).
		Model(&models.Document{}).
		Where("id = ?", documentID).
		Where("proccessing_status NOT IN ?", []string{"processing", "completed"}).
		Updates(map[string]any{
			"status":             "processing",
			"proccessing_status": "processing",
			"updated_at":         gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

func (ds *DocumentStore) CreateChunks(ctx context.Context, chunks []models.DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	return ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("document_id = ?", chunks[0].DocumentID).
			Delete(&models.DocumentChunk{}).Error; err != nil {
			return err
		}

		return tx.
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "document_id"},
					{Name: "chunk_index"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"content",
					"token_size",
					"start_sentence",
					"end_sentence",
					"embedding",
					"embedding_model",
					"embedding_dim",
					"metadata",
					"updated_at",
				}),
			}).
			Create(&chunks).Error
	})
}

func (ds *DocumentStore) UpdateDocumentProcessingResult(ctx context.Context, documentID uuid.UUID, status string, processingStatus string, chunkCount int, classification string, confidence float64, summary string) error {
	updates := map[string]any{
		"status":                    status,
		"proccessing_status":        processingStatus,
		"chunk_count":               chunkCount,
		"classification":            classification,
		"classification_confidence": confidence,
		"summary":                   summary,
		"updated_at":                gorm.Expr("NOW()"),
	}

	return ds.db.WithContext(ctx).
		Model(&models.Document{}).
		Where("id = ?", documentID).
		Updates(updates).Error
}

func (ds *DocumentStore) SearchDocumentChunks(
	ctx context.Context,
	documentID uuid.UUID,
	queryEmbedding []float64,
	limit int,
) ([]models.RetrievedDocumentChunk, error) {
	if limit <= 0 {
		limit = 8
	}

	var chunks []models.RetrievedDocumentChunk

	err := ds.db.WithContext(ctx).
		Raw(`
	SELECT
    document_chunks.document_id,
    document_chunks.chunk_index,
    document_chunks.content,
    document_chunks.metadata,
    document_chunks.embedding <=> CAST(? AS vector) AS distance
FROM document_chunks
JOIN documents ON documents.id = document_chunks.document_id
WHERE document_chunks.document_id = ?
  AND documents.deleted_at IS NULL
ORDER BY distance ASC
LIMIT ?
		`, models.Vector(queryEmbedding), documentID, limit).
		Scan(&chunks).Error

	return chunks, err
}
