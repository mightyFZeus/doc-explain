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

	"github.com/dlclark/regexp2"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"golang.org/x/crypto/bcrypt"
)

var passwordRegex = regexp2.MustCompile(`^(?=.*\d)(?=.*[!@#$%^&*()_+{}\[\]:;<>,.?~\\/\-])(?=.*[A-Z])(?=.*[a-z]).{12,}$`, 0)

type contextKey string

const UserContextKey = contextKey("user")

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

func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) error {
	type envelope struct {
		Error string `json:"error"`
	}

	return writeJSON(w, status, &envelope{Error: message})
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
		writeJSONError(w, http.StatusBadRequest, strings.Join(errorMessages, ", "))

		return nil
	}
	writeJSONError(w, http.StatusBadRequest, err.Error())

	return err
}

func DecodeAndValidate(
	w http.ResponseWriter,
	r *http.Request,
	dst interface{},
) error {

	if r.Body == nil || r.ContentLength == 0 {
		err := errors.New("request body must not be empty")
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return err
	}

	if err := readJSON(w, r, dst); err != nil {
		if errors.Is(err, io.EOF) {

			writeJSONError(w, http.StatusBadRequest, err.Error())

			return err
		}

		writeJSONError(w, http.StatusBadRequest, err.Error())

		return err
	}

	validate := validator.New()
	if err := validate.Struct(dst); err != nil {
		ValidatePayload(w, r, err)
		return err
	}

	return nil
}

func GenerateJWT(userID string, email string) (string, error) {
	secretKey := env.GetString("SECRET_KEY", "")

	jwtSecret := []byte(secretKey)
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  email,
		"exp":    time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func GetUserFromContext(ctx context.Context) (UserClaims, error) {
	user, ok := ctx.Value(UserContextKey).(UserClaims)
	if !ok {
		return UserClaims{}, errors.New("user not found in context")
	}
	return user, nil
}

// IsValidPasswordPCRE checks if the password meets strength requirements
func IsValidPasswordPCRE(password string) bool {
	match, err := passwordRegex.MatchString(password)
	if err != nil {
		return false
	}
	return match
}
