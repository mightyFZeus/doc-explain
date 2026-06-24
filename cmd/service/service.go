package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/teilomillet/raggo"
)

type Service struct {
	Embedder       *raggo.EmbeddingService
	OpenAI         *openai.Client
	LLMModel       string
	TopK           int
	Chunker        raggo.Chunker
	EmbeddingModel string
}

func NewService() (*Service, error) {
	apiKey := env.GetString("OPENAI_API_KEY", "")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is required")
	}
	embeddingModel := env.GetString("OPEN_AI_EMBEDDING_MODEL", "")
	if embeddingModel == "" {
		return nil, errors.New("OPEN_AI_EMBEDDING_MODEL is required")
	}
	embeddingProvider := env.GetString("MODEL_PROVIDER", "")
	if embeddingProvider == "" {
		return nil, errors.New("MODEL_PROVIDER is required")
	}
	chunker, err := raggo.NewChunker(
		raggo.ChunkSize(250),
		raggo.ChunkOverlap(40),
		raggo.WithSentenceSplitter(raggo.SmartSentenceSplitter()),
	)
	if err != nil {
		return nil, fmt.Errorf("create raggo chunker: %w", err)
	}

	embedder, err := raggo.NewEmbedder(
		raggo.SetEmbedderAPIKey(apiKey),
		raggo.SetEmbedderModel(embeddingModel),
		raggo.SetEmbedderProvider(embeddingProvider),
		raggo.SetOption("timeout", 2*time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("create raggo embedder: %w", err)
	}

	openaiClient := openai.NewClient(option.WithAPIKey(apiKey))

	return &Service{
		Embedder:       raggo.NewEmbeddingService(embedder),
		LLMModel:       env.GetString("OPEN_AI_MODEL", ""),
		TopK:           8,
		Chunker:        chunker,
		OpenAI:         &openaiClient,
		EmbeddingModel: embeddingModel,
	}, nil
}
