package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/cmd/helpers"
	"golang.org/x/time/rate"
)

func (app *application) AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenStr := ""
			if authHeader != "" {
				parts := strings.Fields(authHeader)
				if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
					app.unauthorizedResponse(w, r, errors.New("invalid Authorization header format"))

					return
				}
				tokenStr = parts[1]
			}

			if tokenStr == "" && isWebSocketRequest(r) {
				tokenStr = strings.TrimSpace(r.URL.Query().Get("token"))
			}

			if tokenStr == "" {
				app.unauthorizedResponse(w, r, errors.New("missing Authorization header"))

				return
			}

			claims := &helpers.UserClaims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("invalid signing method")
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				app.unauthorizedResponse(w, r, errors.New("invalid token"))
				return
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				app.unauthorizedResponse(w, r, errors.New("invalid user id"))
				return
			}

			user := helpers.UserClaims{
				UserID: userID.String(),
				Email:  claims.Email,
				Name:   claims.Name,
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
			}

			ctx := context.WithValue(r.Context(), helpers.UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// concurrency middleware:Prevents simultaneous requests per user
func (app *application) ConcurrencyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWebSocketRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			userCtx := r.Context().Value(helpers.UserContextKey)
			if userCtx == nil {
				app.unauthorizedResponse(w, r, errors.New("user not authenticated"))
				return
			}
			user := userCtx.(helpers.UserClaims)

			lockIface, _ := app.middleWare.userLocks.LoadOrStore(user.UserID, &sync.Mutex{})
			lock := lockIface.(*sync.Mutex)

			locked := make(chan struct{})
			go func() {
				lock.Lock()
				close(locked)
			}()

			select {
			case <-locked:
				defer lock.Unlock()
				next.ServeHTTP(w, r)
			case <-time.After(2 * time.Second):
				app.tooManyRequests(w, r, errors.New("too many requests"))
				return
			}
		})
	}
}

// middleware for rate limiting : 	Limits frequency of requests per user
func (app *application) getRateLimiter(userID string) *rate.Limiter {
	app.middleWare.rlMu.Lock()
	defer app.middleWare.rlMu.Unlock()

	if limiter, exists := app.middleWare.rateLimiters[userID]; exists {
		return limiter
	}

	limiter := rate.NewLimiter(1, 1) // 1 req/sec, burst up to 5
	app.middleWare.rateLimiters[userID] = limiter
	return limiter
}

func (app *application) RateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWebSocketRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			userCtx := r.Context().Value(helpers.UserContextKey)
			if userCtx == nil {
				app.unauthorizedResponse(w, r, errors.New("user not authenticated"))
				return
			}
			user := userCtx.(helpers.UserClaims)
			limiter := app.getRateLimiter(user.UserID)

			if !limiter.Allow() {
				app.tooManyRequests(w, r, errors.New("too many requests"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
