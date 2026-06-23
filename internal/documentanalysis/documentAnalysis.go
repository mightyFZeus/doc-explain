package documentanalysis

import "strings"

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
