package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userContextKey = contextKey("user")

type UserClaims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

func readJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1_048_578 // 1mb
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	return decoder.Decode(data)
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func ValidatePayload(w http.ResponseWriter, r *http.Request, err error) error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		var errorMessages []string
		for _, e := range ve {
			switch e.Tag() {
			case "required":
				errorMessages = append(errorMessages, fmt.Sprintf("%s is required", e.Field()))
			case "oneof":
				errorMessages = append(errorMessages, fmt.Sprintf(
					"%s must be one of [%s]", e.Field(), e.Param(),
				))
			case "eq":
				errorMessages = append(errorMessages,
					fmt.Sprintf("%s must be accepted", e.Field()))
			default:
				errorMessages = append(errorMessages, fmt.Sprintf(
					"%s is invalid (%s)", e.Field(), e.Tag(),
				))
			}
		}

		// http.Error(w, strings.Join(errorMessages, ", ")), http.StatusBadRequest)
		http.Error(w, strings.Join(errorMessages, ", "), http.StatusBadRequest)

		return nil
	}

	http.Error(w, err.Error(), http.StatusBadRequest)
	return err
}

func DecodeAndValidate(
	w http.ResponseWriter,
	r *http.Request,
	dst interface{},
) error {

	if r.Body == nil || r.ContentLength == 0 {
		err := errors.New("request body must not be empty")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	if err := readJSON(w, r, dst); err != nil {
		if errors.Is(err, io.EOF) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	validate := validator.New()
	if err := validate.Struct(dst); err != nil {
		ValidatePayload(w, r, err)
		return err
	}

	return nil
}

func GenerateJWT(userID uuid.UUID, email, name string, role string) (string, error) {
	secretKey := env.GetString("SECRET_KEY", "")

	jwtSecret := []byte(secretKey)
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  email,
		"name":   name,
		"exp":    time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func GetUserFromContext(ctx context.Context) (UserClaims, error) {
	user, ok := ctx.Value(userContextKey).(UserClaims)
	if !ok {
		return UserClaims{}, errors.New("user not found in context")
	}
	return user, nil
}
