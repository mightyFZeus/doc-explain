package helpers

import (
	"fmt"

	"github.com/mightyfzeus/doc-explain/internal/models"
)

func GetPrompt(history []models.DocumentMessage, contextText string, query string) string {
	return fmt.Sprintf(`
You answer questions using only the provided document excerpts.

Conversation history is only for understanding follow-up questions.
Do not use conversation history as factual source unless the document excerpts support it.

Conversation history:
%s

Document excerpts:
%s

Current question:
%s

Rules:
1. Use only the document excerpts as the source of truth.
2. Do not mention chunk numbers.
3. For legal documents, cite sections or clauses from the document when available.
4. If only part of the answer is supported, answer that part and say what was not found.

Answer:
`, FormatMessagesForPrompt(history), contextText, query)
}
