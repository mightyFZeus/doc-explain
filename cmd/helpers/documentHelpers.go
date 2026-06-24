package helpers

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/cmd/service"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"github.com/openai/openai-go/v2"
)

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

func FormatChunksForPrompt(chunks []models.RetrievedDocumentChunk) string {
	var b strings.Builder

	for _, chunk := range chunks {
		fmt.Fprintf(&b, "[chunk %d | distance %.4f]\n%s\n\n", chunk.ChunkIndex, chunk.Distance, chunk.Content)
	}

	return b.String()
}

func VerifyCloudinaryWebhook(body []byte, timestamp int64, signature, apiSecret string) bool {
	var minifiedBuffer bytes.Buffer
	if err := json.Compact(&minifiedBuffer, body); err != nil {
		return false
	}

	payloadStr := minifiedBuffer.String() + strconv.FormatInt(timestamp, 10) + apiSecret

	hash := sha1.Sum([]byte(payloadStr))
	expectedSignature := hex.EncodeToString(hash[:])

	return subtle.ConstantTimeCompare(
		[]byte(expectedSignature),
		[]byte(signature),
	) == 1
}

func DocumentIDFromPublicID(publicID string) (uuid.UUID, error) {
	value := strings.TrimSpace(publicID)
	if index := strings.LastIndex(value, "/"); index >= 0 {
		value = value[index+1:]
	}

	documentID, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errors.New("invalid cloudinary public_id")
	}

	return documentID, nil
}

func RewriteQueryForRetrieval(ctx context.Context, query string, history []models.DocumentMessage) (string, error) {
	if len(history) == 0 {
		return query, nil
	}

	service, err := service.NewService()
	if err != nil {
		return query, err
	}

	completion, err := service.OpenAI.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(service.LLMModel),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(fmt.Sprintf(`
Rewrite the user's latest question as a standalone search query.

Use the conversation only to resolve references like "it", "that", "he", "the section", or "what about this".
Do not answer the question.

Conversation:
%s

Latest question:
%s

Standalone query:
`, FormatMessagesForPrompt(history), query)),
		},
		MaxTokens: openai.Int(120),
	})
	if err != nil || len(completion.Choices) == 0 {
		return query, err
	}

	rewritten := strings.TrimSpace(completion.Choices[0].Message.Content)
	if rewritten == "" {
		return query, nil
	}

	return rewritten, nil
}

func FormatMessagesForPrompt(messages []models.DocumentMessage) string {
	var b strings.Builder

	for _, message := range messages {
		fmt.Fprintf(&b, "%s: %s\n", message.Role, message.Content)
	}

	return b.String()
}
