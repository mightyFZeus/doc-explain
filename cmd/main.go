package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/mightyfzeus/doc-explain/internal/db"
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
		addr = port
	}

	cfg := config{
		addr:   ":8080",
		apiUrl: env.GetString("API_URL", "localhost:8080"),
		db: dbConfig{
			dbAddr:       env.GetString("DB_ADDR", "postgres://admin:adminpassword@localhost:5433/doc-explain-db?sslmode=disable"),
			maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 5),
			maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 2),
			maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "5m"),
		},
		env: env.GetString("ENV", "development"),
		redis: redisConfig{
			url:      env.GetString("REDIS_URL", "redis://localhost:6379"),
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

	app := &application{
		config: cfg,
		logger: logger,
		middleWare: middleWareConfig{
			rateLimiters: make(map[string]*rate.Limiter),
		},

		store: store,
		cld:   cld,
		redis: redis,
		asynqClient:asynqClient,
	}

	mux := app.mount()
	// go app.EmbedDocuments()
	logger.Fatal(app.run(mux))
}
