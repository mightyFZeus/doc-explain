package main

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/mightyfzeus/doc-explain/internal/db"
	"github.com/mightyfzeus/doc-explain/internal/documentworker"
	"github.com/mightyfzeus/doc-explain/internal/env"
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
	cld, _ := db.InitCloudinary()

	redisClient, err := db.ConnectToRedis(redisOpt.Addr, redisOpt.Username, redisOpt.Password, redisOpt.DB)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	concurrency := env.GetInt("DOCUMENT_WORKER_CONCURRENCY", documentworker.DefaultConcurrency)
	if err := documentworker.Run(context.Background(), documentworker.Config{
		RedisOpt:    redisOpt,
		Store:       store,
		Logger:      logger,
		Redis:       redisClient,
		Cloudinary:  cld,
		Concurrency: concurrency,
	}); err != nil {
		log.Fatal(err)
	}
}
