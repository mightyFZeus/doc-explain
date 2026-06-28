package documentworker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	svcservice "github.com/mightyfzeus/doc-explain/cmd/service"
	"github.com/mightyfzeus/doc-explain/internal/documentanalysis"
	"github.com/mightyfzeus/doc-explain/internal/dtos"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"github.com/mightyfzeus/doc-explain/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/teilomillet/raggo"
	"go.uber.org/zap"
)

type DocumentProcessor struct {
	store          store.Storage
	logger         *zap.SugaredLogger
	loader         raggo.Loader
	chunker        raggo.Chunker
	embedding      *raggo.EmbeddingService
	embeddingModel string
	service        *svcservice.Service
	redis          *redis.Client
	cloudinary     *cloudinary.Cloudinary
}

func NewDocumentProcessor(store store.Storage, logger *zap.SugaredLogger, redis *redis.Client, svc *svcservice.Service, cld *cloudinary.Cloudinary) (*DocumentProcessor, error) {
	if svc == nil {
		var err error
		svc, err = svcservice.NewService()
		if err != nil {
			logger.Errorf("error", "Failed to create service", err)
			return nil, err
		}
	}

	loaderTimeoutMinutes := env.GetInt("DOCUMENT_LOADER_TIMEOUT_MINUTES", 15)
	if loaderTimeoutMinutes <= 0 {
		loaderTimeoutMinutes = 15
	}

	return &DocumentProcessor{
		store:          store,
		logger:         logger,
		loader:         raggo.NewLoader(raggo.SetLoaderTimeout(time.Duration(loaderTimeoutMinutes) * time.Minute)),
		embedding:      svc.Embedder,
		chunker:        svc.Chunker,
		embeddingModel: svc.EmbeddingModel,
		service:        svc,
		redis:          redis,
		cloudinary:     cld,
	}, nil
}

func (p *DocumentProcessor) ProcessTask(ctx context.Context, task *asynq.Task) (err error) {
	var payload jobs.ProcessDocumentPayload
	var documentID uuid.UUID

	defer func() {
		if err == nil || documentID == uuid.Nil {
			return
		}
		p.markDocumentFailed(documentID, err)
	}()

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("invalid process document payload: %w: %v", asynq.SkipRetry, err)
	}

	if payload.DocumentID == "" {
		return fmt.Errorf("documentId is required: %w", asynq.SkipRetry)
	}

	if payload.AssetID == "" {
		return fmt.Errorf("assetId is required: %w", asynq.SkipRetry)
	}

	if payload.PublicID == "" {
		return fmt.Errorf("publicId is required: %w", asynq.SkipRetry)
	}

	if payload.SecureURL == "" {
		return fmt.Errorf("secureUrl is required: %w", asynq.SkipRetry)
	}

	documentID, err = documentanalysis.DocumentIDFromPayload(payload.DocumentID)
	if err != nil {
		return fmt.Errorf("invalid document id: %w: %v", asynq.SkipRetry, err)
	}

	p.logger.Infow("document processing started",
		"documentId", documentID.String(),
		"publicId", payload.PublicID,
		"format", payload.Format,
		"bytes", payload.Bytes,
		"pages", payload.Pages,
	)

	documentRecord, err := p.store.Documents.GetDocumentByID(ctx, documentID)
	if err != nil {
		return fmt.Errorf("get document before processing: %w", err)
	}

	documentOwner, err := p.store.Users.GetByID(ctx, documentRecord.UserID)
	if err != nil {
		return fmt.Errorf("get document owner before processing: %w", err)
	}
	actorType := models.ActorTypeForAccount(documentOwner.AccountType)

	parser := documentanalysis.ParserForPayload(payload)

	filePath, err := p.loader.LoadURL(ctx, payload.SecureURL)
	if err != nil {
		return fmt.Errorf("load cloudinary document: %w", err)
	}
	p.logger.Infow("document loaded from cloudinary", "documentId", documentID.String(), "filePath", filePath)

	document, err := parser.Parse(filePath)
	var text string

	if err != nil {
		text, err = documentanalysis.ExtractTextWithOpenAI(ctx, filePath)
		if err != nil {
			return fmt.Errorf("parse document with openai fallback: %w: %v", asynq.SkipRetry, err)
		}
	} else {
		text = document.Content
	}
	if err != nil {
		return fmt.Errorf("parse document: %w: %v", asynq.SkipRetry, err)
	}
	p.logger.Infow("document text extracted", "documentId", documentID.String(), "characters", len(text))

	classification, confidence := documentanalysis.ClassifyDocument(text)
	summary := documentanalysis.SummarizeDocument(text)

	chunks := p.chunker.Chunk(text)
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks produced for document %s: %w", payload.DocumentID, asynq.SkipRetry)
	}
	p.logger.Infow("document chunked", "documentId", documentID.String(), "chunkCount", len(chunks))

	maxChunks := env.GetInt("DOCUMENT_MAX_CHUNKS", 600)
	if maxChunks > 0 && len(chunks) > maxChunks {
		return fmt.Errorf("document produced %d chunks, above max %d: %w", len(chunks), maxChunks, asynq.SkipRetry)
	}

	embeddedChunks, err := p.embedChunksWithRetry(ctx, chunks, 3)
	if err != nil {
		return fmt.Errorf("embed document chunks: %w", err)
	}
	p.logger.Infow("document chunks embedded", "documentId", documentID.String(), "chunkCount", len(embeddedChunks))

	if len(chunks) != len(embeddedChunks) {
		return fmt.Errorf("chunk count mismatch: chunks=%d embedded_chunks=%d", len(chunks), len(embeddedChunks))
	}

	rows := make([]models.DocumentChunk, 0, len(embeddedChunks))
	for i := range embeddedChunks {
		if embeddedChunks[i].Metadata == nil {
			embeddedChunks[i].Metadata = map[string]interface{}{}
		}

		embeddedChunks[i].Metadata["document_id"] = payload.DocumentID
		embeddedChunks[i].Metadata["asset_id"] = payload.AssetID
		embeddedChunks[i].Metadata["public_id"] = payload.PublicID
		embeddedChunks[i].Metadata["source_url"] = payload.SecureURL
		embeddedChunks[i].Metadata["format"] = payload.Format
		embeddedChunks[i].Metadata["chunk_index"] = i

		vector, ok := embeddedChunks[i].Embeddings["default"]
		if !ok {
			for _, candidate := range embeddedChunks[i].Embeddings {
				vector = candidate
				ok = true
				break
			}
		}
		if !ok || len(vector) == 0 {
			return fmt.Errorf("missing embedding for chunk %d", i)
		}
		if len(vector) != 1536 {
			return fmt.Errorf("invalid embedding dimension for chunk %d: got %d, want 1536", i, len(vector))
		}

		metadata, err := json.Marshal(embeddedChunks[i].Metadata)
		if err != nil {
			return fmt.Errorf("marshal chunk metadata: %w", err)
		}

		content := embeddedChunks[i].Text
		if content == "" {
			content = chunks[i].Text
		}
		encryptedContent, err := p.service.ChunkCipher.Encrypt(content)
		if err != nil {
			return fmt.Errorf("encrypt chunk content: %w", err)
		}

		rows = append(rows, models.DocumentChunk{
			DocumentID:     documentID,
			ChunkIndex:     i,
			Content:        encryptedContent,
			TokenSize:      chunks[i].TokenSize,
			StartSentence:  chunks[i].StartSentence,
			EndSentence:    chunks[i].EndSentence,
			Embedding:      models.Vector(vector),
			EmbeddingModel: p.embeddingModel,
			EmbeddingDim:   len(vector),
			Metadata:       json.RawMessage(metadata),
		})
	}

	if err := p.store.Documents.CreateChunks(ctx, rows); err != nil {
		return err
	}
	p.logger.Info("created document chunks", zap.Int("count", len(rows)))

	chunksDedupeKey := fmt.Sprintf("%s:%s", models.EventChunksCreated, documentID.String())
	p.trackAnalytics(ctx, models.AnalyticsEvent{
		EventType:  models.EventChunksCreated,
		ActorType:  actorType,
		UserID:     &documentRecord.UserID,
		DocumentID: &documentID,
		Count:      int64(len(rows)),
		DedupeKey:  &chunksDedupeKey,
	})

	if err := p.store.Documents.UpdateDocumentProcessingResult(
		ctx,
		documentID,
		"ready",
		"completed",
		len(rows),
		classification,
		confidence,
		summary,
	); err != nil {
		p.publishDocumentStatus(ctx, dtos.DocumentStatusEvent{
			DocumentID:       documentID.String(),
			Status:           "failed",
			ProcessingStatus: "failed",
			Error:            err.Error(),
			UpdatedAt:        time.Now(),
		})
		return err
	}

	classificationMetadata, _ := json.Marshal(map[string]any{
		"classification": classification,
		"confidence":     confidence,
	})
	classificationDedupeKey := fmt.Sprintf("%s:%s", models.EventDocumentClassified, documentID.String())
	p.trackAnalytics(ctx, models.AnalyticsEvent{
		EventType:  models.EventDocumentClassified,
		ActorType:  actorType,
		UserID:     &documentRecord.UserID,
		DocumentID: &documentID,
		DedupeKey:  &classificationDedupeKey,
		Metadata:   classificationMetadata,
	})

	p.logger.Infow("document processing completed",
		"documentId", documentID.String(),
		"userId", documentRecord.UserID.String(),
		"status", "ready",
		"processingStatus", "completed",
		"chunkCount", len(rows),
		"classification", classification,
		"classificationConfidence", confidence,
	)

	p.publishDocumentStatus(ctx, dtos.DocumentStatusEvent{
		DocumentID:       documentID.String(),
		Status:           "ready",
		ProcessingStatus: "completed",
		ChunkCount:       len(rows),
		UpdatedAt:        time.Now(),
	})
	p.deleteCloudinaryOriginal(ctx, payload)

	return nil
}

func (p *DocumentProcessor) markDocumentFailed(documentID uuid.UUID, processingErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errMessage := processingErr.Error()
	if statusErr := p.store.Documents.UpdateDocumentProcessingStatus(ctx, documentID, "failed", "failed", 0); statusErr != nil {
		p.logger.Errorw("failed to mark document as failed",
			"documentId", documentID.String(),
			"processingError", errMessage,
			"statusError", statusErr,
		)
		return
	}

	p.logger.Errorw("document processing failed",
		"documentId", documentID.String(),
		"error", errMessage,
	)

	p.publishDocumentStatus(ctx, dtos.DocumentStatusEvent{
		DocumentID:       documentID.String(),
		Status:           "failed",
		ProcessingStatus: "failed",
		Error:            errMessage,
		UpdatedAt:        time.Now(),
	})
}

func (p *DocumentProcessor) deleteCloudinaryOriginal(ctx context.Context, payload jobs.ProcessDocumentPayload) {
	if p.cloudinary == nil {
		p.logger.Warnw("cloudinary client unavailable; original document was not deleted", "documentId", payload.DocumentID)
		return
	}

	resourceType := strings.TrimSpace(payload.ResourceType)
	if resourceType == "" {
		resourceType = "raw"
	}

	invalidate := true
	result, err := p.cloudinary.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:     payload.PublicID,
		ResourceType: resourceType,
		Invalidate:   &invalidate,
	})
	if err != nil {
		p.logger.Errorw("failed to delete original document from cloudinary",
			"documentId", payload.DocumentID,
			"publicId", payload.PublicID,
			"resourceType", resourceType,
			"error", err,
		)
		return
	}

	p.logger.Infow("deleted original document from cloudinary",
		"documentId", payload.DocumentID,
		"publicId", payload.PublicID,
		"resourceType", resourceType,
		"result", result.Result,
	)
}

func (p *DocumentProcessor) embedChunksWithRetry(ctx context.Context, chunks []raggo.Chunk, attempts int) ([]raggo.EmbeddedChunk, error) {
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		embeddedChunks, err := p.embedding.EmbedChunks(ctx, chunks)
		if err == nil {
			return embeddedChunks, nil
		}

		lastErr = err
		if attempt == attempts {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 2 * time.Second):
		}
	}

	return nil, lastErr
}

func (p *DocumentProcessor) publishDocumentStatus(ctx context.Context, event dtos.DocumentStatusEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		p.logger.Errorw("failed to marshal document status event", "error", err)
		return
	}

	if err := p.redis.Publish(ctx, dtos.DocumentStatusChannel, payload).Err(); err != nil {
		p.logger.Errorw("failed to publish document status", "error", err)
	}
}
