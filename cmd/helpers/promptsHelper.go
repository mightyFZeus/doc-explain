package helpers

import (
	"fmt"

	"github.com/mightyfzeus/doc-explain/internal/models"
)

func GetPrompt(history []models.DocumentMessage, contextText string, query string) string {
	return fmt.Sprintf(`
You are an expert assistant that answers questions accurately using only the provided document excerpts.

Instructions for Citations:
1. Every factual claim you make must be immediately followed by an inline citation to the source excerpt number, formatted exactly like, [2], etc. (e.g., "The contract terminates in 30 days [1].").
2. If multiple excerpts support a claim, combine them (e.g., "[1][3]").
3. For legal documents, include both the source excerpt number and the specific section or clause if visible in the text (e.g., "[1, Section 4.2]").
4. Never make a claim without a citation.

Conversation history (Use ONLY to understand context/follow-ups):
%s

Document excerpts (Each excerpt starts with a clear Identifier like [Excerpt X]):
%s

Current question:
%s

Strict Rules:
1. Rely ONLY on the clear facts directly mentioned in the document excerpts. 
2. Do not assume, extrapolate, or bring in outside knowledge.
3. If only part of the answer is supported, answer that part with citations, and state exactly what information was missing from the excerpts.
4. Do not mention specific raw database chunk numbers, only use the [Excerpt X] identifiers provided below.

Answer:
`, FormatMessagesForPrompt(history), contextText, query)
}
