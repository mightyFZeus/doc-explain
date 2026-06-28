package documentanalysis

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/responses"
	"github.com/teilomillet/raggo"
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

func ExtractTextWithOpenAI(ctx context.Context, filePath string) (string, error) {
	apiKey := env.GetString("OPENAI_API_KEY", "")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is required")
	}
	model := env.GetString("OPEN_AI_FILE_EXTRACTION_MODEL", env.GetString("OPEN_AI_MODEL", ""))
	if model == "" {
		return "", fmt.Errorf("OPEN_AI_MODEL or OPEN_AI_FILE_EXTRACTION_MODEL is required")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open pdf for openai extraction: %w", err)
	}
	defer f.Close()

	client := openai.NewClient(option.WithAPIKey(apiKey))

	file, err := client.Files.New(ctx, openai.FileNewParams{
		File:    f,
		Purpose: openai.FilePurposeUserData,
	})
	if err != nil {
		return "", fmt.Errorf("upload file to openai: %w", err)
	}

	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: responses.ResponsesModel(model),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(`
Extract all readable text from this document.

Rules:
- Return only clean plain text.
- Preserve headings, paragraphs, bullets, and tables as readable text.
- Include page breaks as: --- page N ---
- Do not summarize.
- Do not add commentary.
`),
						{
							OfInputFile: &responses.ResponseInputFileParam{
								FileID: openai.String(file.ID),
							},
						},
					},
					responses.EasyInputMessageRoleUser,
				),
			},
		},
		MaxOutputTokens: openai.Int(12000),
	})
	if err != nil {
		return "", fmt.Errorf("extract text with openai: %w", err)
	}

	text := strings.TrimSpace(resp.OutputText())
	if text == "" {
		return "", fmt.Errorf("openai returned empty file extraction")
	}

	return text, nil
}

func ParserForPayload(payload jobs.ProcessDocumentPayload) raggo.Parser {
	format := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(payload.Format)), ".")
	filename := strings.ToLower(payload.OriginalFilename)
	url := strings.ToLower(payload.SecureURL)

	switch {
	case format == "pdf" || strings.HasSuffix(filename, ".pdf") || strings.HasSuffix(url, ".pdf"):
		return raggo.PDFParser()
	case format == "txt" ||
		format == "text" ||
		format == "md" ||
		format == "markdown" ||
		strings.HasSuffix(filename, ".txt") ||
		strings.HasSuffix(filename, ".md") ||
		strings.HasSuffix(filename, ".markdown") ||
		strings.Contains(url, "/raw/upload/"):
		return raggo.TextParser()
	default:
		return raggo.NewParser()
	}
}

func DocumentIDFromPayload(value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if index := strings.LastIndex(value, "/"); index >= 0 {
		value = value[index+1:]
	}

	return uuid.Parse(value)
}
