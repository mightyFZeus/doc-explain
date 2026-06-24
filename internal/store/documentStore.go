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
    document_id,
    chunk_index,
    content,
    metadata,
    embedding <=> CAST(? AS vector) AS distance
FROM document_chunks
WHERE document_id = ?
  -- Corrected line: No "AS" alias used here
  AND (embedding <=> CAST(? AS vector)) < 0.6 
ORDER BY distance ASC
LIMIT ?
		`, models.Vector(queryEmbedding), documentID, models.Vector(queryEmbedding), limit).
		Scan(&chunks).Error

	return chunks, err
}
