package helpers

import (
	"fmt"
	"strings"

	"github.com/mightyfzeus/doc-explain/internal/models"
)

func GetPrompt(history []models.DocumentMessage, contextText string, query string, isLegalDocument bool) string {
	if IsDocumentDraftingQuery(query) {
		return fmt.Sprintf(`
You are an expert writing assistant. Draft the requested document using only the provided document excerpts and the current question.

The source document may be a CV/resume. Before drafting, infer the candidate's available basic details from the excerpts, including name, email, phone, location, current/target role, education, skills, and work history.

If a "Candidate profile extracted from the document" block is present, use those fields as the applicant's basic information.

Conversation history is only for understanding follow-up instructions:
%s

Document excerpts:
%s

Current request:
%s

Strict Rules:
1. Use only facts directly supported by the document excerpts.
2. Do not use bracket placeholders for candidate details that appear in the document or candidate profile block. For example, do not write [Your Name] if the candidate name is present.
3. If employer, hiring manager, company, job title, address, or date are not provided, do not invent them and do not use bracket placeholders. Use natural generic wording such as "Dear Hiring Manager," and "your team" where appropriate.
4. Do not include chunk numbers, excerpt numbers, source numbers, or citations in drafted artifacts like cover letters.
5. Never output template fields like [Your Name], [Your Address], [Date], [Email Address], [Phone Number], [Employer's Name], [Company's Name], or [Position Title].
6. Do not output square-bracket placeholder text anywhere in the draft.
7. If the candidate profile contains a name, the first applicant name line must be the actual candidate name, not a placeholder followed by the candidate name.
8. Do not add a note telling the user to modify placeholders.
9. If the document does not contain enough candidate information to draft the requested artifact, ask for the missing information briefly instead of producing a placeholder-heavy draft.

Draft:
`, FormatMessagesForPrompt(history), contextText, query)
	}

	if isLegalDocument {
		return fmt.Sprintf(`
You are an expert assistant that answers legal-document questions accurately using only the provided document excerpts.

Conversation history (Use ONLY to understand context/follow-ups):
%s

Document excerpts:
%s

Current question:
%s

Strict Rules:
1. Rely ONLY on the clear facts directly mentioned in the document excerpts.
2. Do not assume, extrapolate, or bring in outside knowledge.
3. If only part of the answer is supported, answer that part and state exactly what information was missing from the excerpts.
4. Cite legal authority by visible section, subsection, paragraph, article, clause, schedule, or regulation names/numbers from the document when available.
5. Do not cite chunk numbers, excerpt numbers, source numbers, raw database identifiers, or bracketed numeric references like [0], [1], or [2].
6. If no section or clause is visible in the excerpts, answer without inventing a citation.

Answer:
`, FormatMessagesForPrompt(history), contextText, query)
	}

	return fmt.Sprintf(`
You are an expert assistant that answers questions accurately using only the provided document excerpts.

Conversation history (Use ONLY to understand context/follow-ups):
%s

Document excerpts:
%s

Current question:
%s

Strict Rules:
1. Rely ONLY on the clear facts directly mentioned in the document excerpts. 
2. Do not assume, extrapolate, or bring in outside knowledge.
3. If only part of the answer is supported, answer that part and state exactly what information was missing from the excerpts.
4. Do not mention chunk numbers, excerpt numbers, source numbers, or raw database identifiers.
5. Do not include citations unless the document is legal.
6. This document is not legal, so do not include bracketed numeric references like [0], [1], or [2], and do not cite excerpts, chunks, or sources.
7. Ignore any citation style used in the conversation history.

Answer:
`, FormatMessagesForPrompt(history), contextText, query)
}

func IsLegalDocument(document models.Document) bool {
	return strings.EqualFold(strings.TrimSpace(document.Classification), "legal")
}
