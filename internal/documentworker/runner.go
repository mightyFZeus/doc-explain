package documentworker

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"
	svcservice "github.com/mightyfzeus/doc-explain/cmd/service"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/store"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const DefaultConcurrency = 10

type Config struct {
	RedisOpt    asynq.RedisClientOpt
	Store       store.Storage
	Logger      *zap.SugaredLogger
	Redis       *redis.Client
	Service     *svcservice.Service
	Concurrency int
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.Logger == nil {
		return errors.New("document worker logger is required")
	}
	if cfg.Redis == nil {
		return errors.New("document worker redis client is required")
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	server := asynq.NewServer(cfg.RedisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues: map[string]int{
			"rag":     10,
			"default": 1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			cfg.Logger.Errorw("asynq task failed",
				"type", task.Type(),
				"payload", string(task.Payload()),
				"error", err,
			)
		}),
	})

	processor, err := NewDocumentProcessor(cfg.Store, cfg.Logger, cfg.Redis, cfg.Service)
	if err != nil {
		return err
	}

	encryptedChunks, err := cfg.Store.Documents.EncryptPlaintextChunks(ctx, processor.service.ChunkCipher.Encrypt)
	if err != nil {
		return err
	}
	if encryptedChunks > 0 {
		cfg.Logger.Infow("encrypted plaintext chunks", "count", encryptedChunks)
	}

	mux := asynq.NewServeMux()
	mux.Handle(jobs.TypeProcessDocument, processor)

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			server.Shutdown()
		case <-done:
		}
	}()

	cfg.Logger.Infow("document queue runner started", "concurrency", concurrency)
	err = server.Run(mux)
	close(done)
	return err
}
