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
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/cmd/service"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"github.com/openai/openai-go/v2"
)

var (
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern = regexp.MustCompile(`(?:\+?\d[\d\s().-]{7,}\d)`)
)

func ClassifyDocument(text string) (string, float64) {
	lower := strings.ToLower(text)
	resumeScore := resumeClassificationScore(text, lower)
	if resumeScore >= 5 {
		return "resume", confidenceFromScore(resumeScore)
	}

	scores := map[string]int{
		"resume":      resumeScore,
		"legal":       countMatches(lower, "agreement", "party", "parties", "clause", "section", "subsection", "law", "act", "regulation", "court", "jurisdiction", "terms and conditions", "governing law", "liability"),
		"research":    countMatches(lower, "abstract", "methodology", "references", "literature review", "findings", "conclusion"),
		"financial":   countMatches(lower, "balance sheet", "income statement", "cash flow", "revenue", "profit", "expenses", "assets"),
		"educational": countMatches(lower, "lesson", "course", "assignment", "curriculum", "learning objectives", "student"),
		"technical":   countMatches(lower, "api", "architecture", "database", "deployment", "authentication", "backend", "frontend", "system design", "infrastructure"),
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

	return bestClass, confidenceFromScore(bestScore)
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

func resumeClassificationScore(original string, lower string) int {
	score := 0

	if emailPattern.MatchString(original) {
		score += 3
	}
	if phonePattern.MatchString(original) {
		score += 2
	}

	score += countMatches(lower,
		"curriculum vitae",
		"resume",
		"cv",
		"profile summary",
		"professional summary",
		"career summary",
		"work experience",
		"professional experience",
		"employment history",
		"employment experience",
		"experience",
		"education",
		"technical skills",
		"skills",
		"certifications",
		"projects",
		"portfolio",
		"linkedin",
		"github",
		"references",
	)

	score += countMatches(lower,
		"software engineer",
		"product engineer",
		"backend engineer",
		"frontend engineer",
		"mobile engineer",
		"full stack",
		"developer",
		"react native",
		"golang",
	)

	return score
}

func confidenceFromScore(score int) float64 {
	confidence := 0.60 + float64(score)*0.05
	if confidence > 0.95 {
		return 0.95
	}
	return confidence
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

	for i, chunk := range chunks {
		fmt.Fprintf(&b, "Excerpt %d:\n%s\n\n", i+1, chunk.Content)
	}

	return b.String()
}

func FormatCandidateProfileForPrompt(chunks []models.RetrievedDocumentChunk) string {
	text := joinedChunkText(chunks)
	lines := meaningfulLines(text)

	name := candidateNameFromLines(lines)
	email := firstMatch(emailPattern, text)
	phone := firstMatch(phonePattern, text)
	role := candidateRoleFromLines(lines)

	if name == "" && email == "" && phone == "" && role == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("Candidate profile extracted from the document:\n")
	if name != "" {
		fmt.Fprintf(&b, "- Candidate name: %s\n", name)
	}
	if role != "" {
		fmt.Fprintf(&b, "- Candidate role/title: %s\n", role)
	}
	if email != "" {
		fmt.Fprintf(&b, "- Email: %s\n", email)
	}
	if phone != "" {
		fmt.Fprintf(&b, "- Phone: %s\n", phone)
	}

	return strings.TrimSpace(b.String())
}

func IsDocumentDraftingQuery(query string) bool {
	normalized := strings.ToLower(query)

	draftingSignals := []string{
		"cover letter",
		"application letter",
		"motivation letter",
		"write a letter",
		"draft a letter",
		"generate a letter",
		"write me a letter",
		"write me cover",
		"generate cover",
		"draft cover",
	}

	for _, signal := range draftingSignals {
		if strings.Contains(normalized, signal) {
			return true
		}
	}

	return false
}

func joinedChunkText(chunks []models.RetrievedDocumentChunk) string {
	var b strings.Builder
	for _, chunk := range chunks {
		if chunk.Content == "" {
			continue
		}
		b.WriteString(chunk.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func meaningfulLines(text string) []string {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))

	for _, line := range rawLines {
		line = strings.TrimSpace(strings.Trim(line, "|•*-_"))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	return lines
}

func candidateNameFromLines(lines []string) string {
	maxLines := 20
	if len(lines) < maxLines {
		maxLines = len(lines)
	}

	for _, line := range lines[:maxLines] {
		candidate := strings.TrimSpace(line)
		lower := strings.ToLower(candidate)
		words := strings.Fields(candidate)

		if len(words) < 2 || len(words) > 5 {
			continue
		}
		if strings.Contains(candidate, "@") || phonePattern.MatchString(candidate) {
			continue
		}
		if strings.Contains(lower, "resume") ||
			strings.Contains(lower, "curriculum") ||
			strings.Contains(lower, "profile") ||
			strings.Contains(lower, "summary") ||
			strings.Contains(lower, "engineer") ||
			strings.Contains(lower, "developer") ||
			strings.Contains(lower, "linkedin") ||
			strings.Contains(lower, "github") ||
			strings.Contains(lower, "portfolio") {
			continue
		}

		letterCount := 0
		for _, r := range candidate {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				letterCount++
			}
		}
		if letterCount < 4 {
			continue
		}

		return candidate
	}

	return ""
}

func candidateRoleFromLines(lines []string) string {
	roleSignals := []string{
		"engineer",
		"developer",
		"designer",
		"manager",
		"analyst",
		"architect",
		"consultant",
	}

	maxLines := 30
	if len(lines) < maxLines {
		maxLines = len(lines)
	}

	for _, line := range lines[:maxLines] {
		lower := strings.ToLower(line)
		for _, signal := range roleSignals {
			if strings.Contains(lower, signal) && len(strings.Fields(line)) <= 10 {
				return line
			}
		}
	}

	return ""
}

func firstMatch(pattern *regexp.Regexp, text string) string {
	match := pattern.FindString(text)
	return strings.TrimSpace(match)
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
