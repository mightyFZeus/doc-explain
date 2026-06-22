package main

import (
	"context"
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

func (app *application) ValidatePayload(w http.ResponseWriter, r *http.Request, err error) error {
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

		app.badRequestResponse(w, r, errors.New(strings.Join(errorMessages, ", ")))
		return nil
	}

	app.badRequestResponse(w, r, err)
	return err
}

func (app *application) DecodeAndValidate(
	w http.ResponseWriter,
	r *http.Request,
	dst interface{},
) error {

	if r.Body == nil || r.ContentLength == 0 {
		err := errors.New("request body must not be empty")
		app.badRequestResponse(w, r, err)
		return err
	}

	if err := readJSON(w, r, dst); err != nil {
		if errors.Is(err, io.EOF) {
			app.badRequestResponse(w, r, errors.New("request body must not be empty"))
			return err
		}

		app.badRequestResponse(w, r, err)
		return err
	}

	validate := validator.New()
	if err := validate.Struct(dst); err != nil {
		app.ValidatePayload(w, r, err)
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
