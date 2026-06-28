package main

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/cmd/service"
	"github.com/mightyfzeus/doc-explain/internal/env"
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

	service *service.Service
	docHub  *service.DocumentStatusHub
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

	origins := []string{
		"http://localhost:3000",
		"https://doc-explain.vercel.app",
	}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(timeoutExceptWebSocket(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	secret := env.GetString("SECRET_KEY", "ahjsjj=")

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		app.notFoundResponse(w, r, errors.New("route not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		app.badRequestResponse(w, r, errors.New("method not allowed"))
	})
	r.Post("/cloudinary/webhook", app.CloudinaryUploadWebhook)

	r.Get("/health", app.HealthHandler)

	r.Route("/v1/auth", func(r chi.Router) {
		r.Post("/register", app.RegisterUser)
		r.Post("/guest", app.CreateGuestSessionHandler)

		r.Post("/login", app.LogingHandler)
	})
	r.Route("/v1", func(r chi.Router) {
		r.Use(
			app.AuthMiddleware(secret),
			app.ConcurrencyMiddleware(),
			app.RateLimitMiddleware(),
		)

		r.Get("/documents", app.GetAllDocumentsHandler)
		r.Post("/document/upload", app.UploadDocumentHandler)
		r.Delete("/document", app.DeleteDocumentHandler)
		r.Get("/document/conversations", app.GetDocumentConversationsHandler)
		r.Post("/document/search", app.SearchThroughDocumentHandler)

		r.Get("/ws/document", app.DocumentStatusWSHandler)

	})

	return r
}

func timeoutExceptWebSocket(timeout time.Duration) func(http.Handler) http.Handler {
	timeoutMiddleware := middleware.Timeout(timeout)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWebSocketRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			timeoutMiddleware(next).ServeHTTP(w, r)
		})
	}
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
