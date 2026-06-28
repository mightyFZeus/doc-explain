package documentanalysis

import "testing"

func TestClassifyDocumentDetectsTechnicalResume(t *testing.T) {
	text := `
Bolarin O. Olabisi
Lagos, Nigeria
bolarinolabisi36@gmail.com
09015752977

Product Engineer
Professional Summary
Software Engineer with over four years of experience building high-performance mobile and backend systems.

Work Experience
Product Engineer at Product Studio HQ
Built RESTful APIs with Go, React Native mobile applications, fintech loan products, and backend services.

Education
Bachelor's degree in Computer Science

Technical Skills
Golang, React Native, Backend, Frontend, API, AWS, PostgreSQL
`

	classification, confidence := ClassifyDocument(text)
	if classification != "resume" {
		t.Fatalf("expected resume classification, got %q with confidence %.2f", classification, confidence)
	}
	if confidence < 0.80 {
		t.Fatalf("expected strong resume confidence, got %.2f", confidence)
	}
}
