package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/cmd/service"
	"github.com/mightyfzeus/doc-explain/internal/documentanalysis"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"github.com/mightyfzeus/doc-explain/internal/store"
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
	service        *service.Service
}

func NewDocumentProcessor(store store.Storage, logger *zap.SugaredLogger) (*DocumentProcessor, error) {
	svc, err := service.NewService()
	if err != nil {
		logger.Errorf("error", "Failed to create service", err)
		return nil, err
	}

	return &DocumentProcessor{
		store:          store,
		logger:         logger,
		loader:         raggo.NewLoader(raggo.SetLoaderTimeout(5 * time.Minute)),
		embedding:      svc.Embedder,
		chunker:        svc.Chunker,
		embeddingModel: svc.EmbeddingModel,
		service:        svc,
	}, nil
}

func (p *DocumentProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload jobs.ProcessDocumentPayload

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

	documentID, err := documentanalysis.DocumentIDFromPayload(payload.DocumentID)
	if err != nil {
		return fmt.Errorf("invalid document id: %w: %v", asynq.SkipRetry, err)
	}

	parser := documentanalysis.ParserForPayload(payload)

	// TODO: Get the document from cloudinary
	filePath, err := p.loader.LoadURL(ctx, payload.SecureURL)
	if err != nil {
		return fmt.Errorf("load cloudinary document: %w", err)
	}

	document, err := parser.Parse(filePath)
	var text string
	// parse document content with open ai fallback, if raggo parser fails

	if err != nil {
		text, err = documentanalysis.ExtractTextWithOpenAI(ctx, payload.SecureURL)
		if err != nil {
			if statusErr := p.store.Documents.UpdateDocumentProcessingStatus(ctx, documentID, "failed", "failed", 0); statusErr != nil {
				return fmt.Errorf("mark document failed after openai parse fallback: %v: original error: %w", statusErr, err)
			}
			return fmt.Errorf("parse document with openai fallback: %w: %v", asynq.SkipRetry, err)
		}
	} else {
		text = document.Content
	}
	if err != nil {
		if statusErr := p.store.Documents.UpdateDocumentProcessingStatus(ctx, documentID, "failed", "failed", 0); statusErr != nil {
			return fmt.Errorf("mark document failed after parse error: %v: original error: %w", statusErr, err)
		}
		return fmt.Errorf("parse document: %w: %v", asynq.SkipRetry, err)
	}

	classification, confidence := documentanalysis.ClassifyDocument(text)
	summary := documentanalysis.SummarizeDocument(text)

	// TODO: handle raggo chunk

	chunks := p.chunker.Chunk(text)
	if len(chunks) == 0 {
		if statusErr := p.store.Documents.UpdateDocumentProcessingStatus(ctx, documentID, "failed", "failed", 0); statusErr != nil {
			return fmt.Errorf("mark document failed after empty chunks: %w", statusErr)
		}
		return fmt.Errorf("no chunks produced for document %s: %w", payload.DocumentID, asynq.SkipRetry)
	}

	// TODO: Store chunks + embeddings in DB
	embeddedChunks, err := p.embedChunksWithRetry(ctx, chunks, 3)
	if err != nil {
		return fmt.Errorf("embed document chunks: %w", err)
	}

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

		rows = append(rows, models.DocumentChunk{
			DocumentID:     documentID,
			ChunkIndex:     i,
			Content:        content,
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

	return p.store.Documents.UpdateDocumentProcessingResult(ctx, documentID, "ready", "completed", len(rows), classification, confidence, summary)
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
