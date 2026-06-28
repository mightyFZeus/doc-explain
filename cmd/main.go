package main

import (
	"context"
	"log"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/mightyfzeus/doc-explain/cmd/service"
	"github.com/mightyfzeus/doc-explain/internal/db"
	"github.com/mightyfzeus/doc-explain/internal/documentworker"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/store"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ Could not load .env file, falling back to defaults")
	}
	addr := env.GetString("ADDR", "")
	if addr == "" {
		port := env.GetString("PORT", "8080")
		addr = ":" + strings.TrimPrefix(port, ":")
	}

	cfg := config{
		addr:   addr,
		apiUrl: env.GetString("API_URL", "localhost:8080"),
		db: dbConfig{
			dbAddr:       env.GetString("DB_ADDR", ""),
			maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 5),
			maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 2),
			maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "5m"),
		},
		env: env.GetString("ENV", "development"),
		redis: redisConfig{
			url:      env.GetString("REDIS_URL", "localhost:6379"),
			password: env.GetString("REDIS_PASSWORD", ""),
			db:       env.GetInt("REDIS_DB", 0),
			username: env.GetString("REDIS_USERNAME", ""),
		},
	}

	// logger
	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	// db
	gormDB, err := db.New(cfg.db.dbAddr, cfg.db.maxOpenConns, cfg.db.maxIdleConns, cfg.db.maxIdleTime)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	cld, _ := db.InitCloudinary()
	redis, err := db.ConnectToRedis(cfg.redis.url, cfg.redis.username, cfg.redis.password, cfg.redis.db)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	asynqClient, err := db.InitAsynqClient(cfg.redis.url, cfg.redis.username, cfg.redis.password, cfg.redis.db)
	if err != nil {
		logger.Fatal("failed to connect asynq client", zap.Error(err))
	}
	defer asynqClient.Close()

	sqlDB, err := gormDB.DB()
	if err != nil {
		logger.Fatal("error getting sqlDb from gormDB", zap.Error(err))
	}
	defer sqlDB.Close()

	if err := store.AutoMigrate(gormDB); err != nil {
		logger.Fatal("error running migrations", zap.Error(err))
	}

	defer sqlDB.Close()
	logger.Info("db conncetion pool established")

	// store
	store := store.NewStorage(gormDB)
	svc, err := service.NewService()
	if err != nil {
		logger.Fatal("failed to create service", zap.Error(err))
	}

	app := &application{
		config: cfg,
		logger: logger,
		docHub: service.NewDocumentStatusHub(),
		middleWare: middleWareConfig{
			rateLimiters: make(map[string]*rate.Limiter),
		},

		store:       store,
		cld:         cld,
		redis:       redis,
		asynqClient: asynqClient,
		service:     svc,
	}

	mux := app.mount()
	go app.ListenForDocumentStatusEvents(context.Background())
	if env.GetBool("PROCESS_JOBS_IN_API", true) {
		concurrency := env.GetInt("DOCUMENT_WORKER_CONCURRENCY", documentworker.DefaultConcurrency)
		go func() {
			if err := documentworker.Run(context.Background(), documentworker.Config{
				RedisOpt: asynq.RedisClientOpt{
					Addr:     cfg.redis.url,
					Username: cfg.redis.username,
					Password: cfg.redis.password,
					DB:       cfg.redis.db,
				},
				Store:       store,
				Logger:      logger,
				Redis:       redis,
				Service:     svc,
				Cloudinary:  cld,
				Concurrency: concurrency,
			}); err != nil {
				logger.Errorw("document queue runner stopped", "error", err)
			}
		}()
	}
	// go app.EmbedDocuments()
	logger.Fatal(app.run(mux))
}
