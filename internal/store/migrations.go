package store

import (
	"github.com/mightyfzeus/doc-explain/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	const lockID int64 = 712943281

	if err := db.Exec("SELECT pg_advisory_lock(?)", lockID).Error; err != nil {
		return err
	}
	defer db.Exec("SELECT pg_advisory_unlock(?)", lockID)

	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS vector;`).Error; err != nil {
		return err
	}

	if err := db.AutoMigrate(&models.User{},
		&models.Document{},
		&models.DocumentChunk{},
		&models.DocumentConversation{},
		&models.DocumentMessage{},
		&models.AnalyticsEvent{}); err != nil {
		return err
	}

	if !db.Migrator().HasTable("documents") {
		return nil
	}

	if db.Migrator().HasColumn("documents", "content") {
		// Ensure documents.tsv exists
		if err := db.Exec(`
			ALTER TABLE documents
			ADD COLUMN IF NOT EXISTS tsv tsvector;
		`).Error; err != nil {
			return err
		}

		// Create GIN index for full-text search
		if err := db.Exec(`
			CREATE INDEX IF NOT EXISTS documents_tsv_idx
			ON documents
			USING gin(tsv);
		`).Error; err != nil {
			return err
		}

		// Create trigger function if needed
		if err := db.Exec(`
			CREATE OR REPLACE FUNCTION documents_tsv_trigger()
			RETURNS trigger AS $$
			BEGIN
				NEW.tsv := to_tsvector('english', COALESCE(NEW.content, ''));
				RETURN NEW;
			END
			$$ LANGUAGE plpgsql;
		`).Error; err != nil {
			return err
		}

		// Create trigger if it doesn't exist
		if err := db.Exec(`
			DO $$
			BEGIN
				IF NOT EXISTS (
					SELECT 1
					FROM pg_trigger
					WHERE tgname = 'documents_tsv_update'
				) THEN
					CREATE TRIGGER documents_tsv_update
					BEFORE INSERT OR UPDATE
					ON documents
					FOR EACH ROW
					EXECUTE FUNCTION documents_tsv_trigger();
				END IF;
			END
			$$;
		`).Error; err != nil {
			return err
		}

		// Backfill existing rows
		if err := db.Exec(`
			UPDATE documents
			SET tsv = to_tsvector('english', COALESCE(content, ''))
			WHERE tsv IS NULL;
		`).Error; err != nil {
			return err
		}
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS document_chunks_embedding_hnsw_idx
		ON document_chunks
		USING hnsw (embedding vector_cosine_ops);
	`).Error; err != nil {
		return err
	}

	return nil
}
