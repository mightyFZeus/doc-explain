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

func ClassifyDocument(text string) (string, float64) {
	lower := strings.ToLower(text)

	scores := map[string]int{
		"resume":      countMatches(lower, "profile summary", "employment history", "skills", "education", "certifications", "linkedin", "resume", "cv"),
		"legal":       countMatches(lower, "agreement", "party", "parties", "clause", "terms and conditions", "governing law", "liability"),
		"research":    countMatches(lower, "abstract", "methodology", "references", "literature review", "findings", "conclusion"),
		"financial":   countMatches(lower, "balance sheet", "income statement", "cash flow", "revenue", "profit", "expenses", "assets"),
		"educational": countMatches(lower, "lesson", "course", "assignment", "curriculum", "learning objectives", "student"),
		"technical":   countMatches(lower, "api", "architecture", "database", "deployment", "authentication", "backend", "frontend"),
	}

	bestClass := "general"
	bestScore := 0
	for class, score := range scores {
		if score > bestScore {
			bestClass = class
			bestScore = score
		}
	}

	if bestScore == 0 {
		return bestClass, 0.50
	}

	confidence := 0.60 + float64(bestScore)*0.08
	if confidence > 0.95 {
		confidence = 0.95
	}

	return bestClass, confidence
}

func countMatches(text string, keywords ...string) int {
	count := 0
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			count++
		}
	}
	return count
}

func SummarizeDocument(text string) string {
	normalized := strings.Join(strings.Fields(text), " ")
	if len(normalized) <= 600 {
		return normalized
	}

	summary := normalized[:600]
	if index := strings.LastIndexAny(summary, ".!?"); index >= 240 {
		return strings.TrimSpace(summary[:index+1])
	}

	return strings.TrimSpace(summary) + "..."
}

func IsAllowedDocumentUpload(fileName string, detectedType string) bool {
	switch {
	case strings.HasSuffix(fileName, ".pdf"):
		return detectedType == "application/pdf"
	case strings.HasSuffix(fileName, ".jpg"), strings.HasSuffix(fileName, ".jpeg"):
		return detectedType == "image/jpeg"
	case strings.HasSuffix(fileName, ".png"):
		return detectedType == "image/png"
	case strings.HasSuffix(fileName, ".md"), strings.HasSuffix(fileName, ".markdown"), strings.HasSuffix(fileName, ".txt"):
		return strings.HasPrefix(detectedType, "text/plain") || strings.HasPrefix(detectedType, "text/markdown")
	case strings.HasSuffix(fileName, ".docx"):
		return detectedType == "application/zip" ||
			detectedType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return false
	}
}
