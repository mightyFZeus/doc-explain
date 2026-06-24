package main

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/mightyfzeus/doc-explain/internal/db"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/mightyfzeus/doc-explain/internal/store"
	"go.uber.org/zap"
)

type dbConfig struct {
	dbAddr       string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ Could not load .env file, falling back to defaults")
	}
	redisOpt := asynq.RedisClientOpt{
		Addr:     env.GetString("REDIS_URL", "localhost:6379"),
		Username: env.GetString("REDIS_USERNAME", ""),
		Password: env.GetString("REDIS_PASSWORD", ""),
		DB:       env.GetInt("REDIS_DB", 0),
	}
	logger := zap.Must(zap.NewProduction()).Sugar()

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 3,
		Queues: map[string]int{
			"rag":     10,
			"default": 1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			logger.Errorw("asynq task failed",
				"type", task.Type(),
				"payload", string(task.Payload()),
				"error", err,
			)
		}),
	})
	// logger
	defer logger.Sync()
	// db
	dbCon := dbConfig{
		dbAddr:       env.GetString("DB_ADDR", "postgres://admin:adminpassword@localhost:5433/doc-explain-db?sslmode=disable"),
		maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 5),
		maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 2),
		maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "5m"),
	}
	gormDB, err := db.New(dbCon.dbAddr, dbCon.maxOpenConns, dbCon.maxIdleConns, dbCon.maxIdleTime)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	store := store.NewStorage(gormDB)

	mux := asynq.NewServeMux()

	processor, err := NewDocumentProcessor(store, logger)
	if err != nil {
		logger.Fatal("failed to create document processor", zap.Error(err))
	}
	encryptedChunks, err := store.Documents.EncryptPlaintextChunks(context.Background(), processor.service.ChunkCipher.Encrypt)
	if err != nil {
		logger.Fatal("failed to encrypt plaintext chunks", zap.Error(err))
	}
	if encryptedChunks > 0 {
		logger.Infow("encrypted plaintext chunks", "count", encryptedChunks)
	}
	mux.Handle(jobs.TypeProcessDocument, processor)
	// start server
	logger.Info("server is running")
	defer server.Shutdown()

	if err := server.Run(mux); err != nil {
		log.Fatal(err)
	}
}
