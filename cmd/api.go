package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/internal/store"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type application struct {
	logger      *zap.SugaredLogger
	config      config
	middleWare  middleWareConfig
	store       store.Storage
	cld         *cloudinary.Cloudinary
	redis       *redis.Client
	asynqClient *asynq.Client
}

type config struct {
	addr   string
	apiUrl string
	db     dbConfig
	env    string
	redis  redisConfig
}

type redisConfig struct {
	url      string
	password string
	db       int
	username string
}

type dbConfig struct {
	dbAddr       string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}
type middleWareConfig struct {
	userLocks    sync.Map
	rateLimiters map[string]*rate.Limiter
	rlMu         sync.Mutex
}

func (app *application) mount() http.Handler {

	origins := []string{}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", app.HealthHandler)
	r.Post("/auth/register", app.RegisterUser)
	r.Post("/document/upload", app.UploadDocumentHandler)
	r.Post("/cloudinary/webhook", app.CloudinaryUploadWebhook)

	return r
}

func (app *application) run(mux http.Handler) error {
	srv := &http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: 70 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	return srv.ListenAndServe()
}
