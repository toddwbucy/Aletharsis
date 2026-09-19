// Package evidence defines the runtime-independent report contract.
package evidence

import "crypto/sha256"
import "fmt"

type Object = map[string]any

type File struct {
	Path      string  `json:"path"`
	Filename  string  `json:"filename"`
	Extension string  `json:"extension"`
	MIME      string  `json:"mime"`
	Format    string  `json:"format"`
	Basis     string  `json:"identification_basis"`
	Size      *int    `json:"size"`
	SHA256    *string `json:"sha256"`
	Parser    *string `json:"parser"`
}
type Text struct {
	Source        string            `json:"source"`
	Text          string            `json:"text"`
	Encoding      string            `json:"encoding"`
	ByteOffsets   []int             `json:"byte_offsets"`
	BOM           *string           `json:"bom"`
	LineEndings   map[string]int    `json:"line_endings"`
	Hashes        map[string]string `json:"hashes"`
	Normalization Object            `json:"normalization"`
}
type Document struct {
	Texts     []*Text `json:"texts"`
	Metadata  Object  `json:"metadata"`
	Structure Object  `json:"structure"`
}
type Finding struct {
	ID             string  `json:"id"`
	Severity       string  `json:"severity"`
	Confidence     float64 `json:"confidence"`
	Category       string  `json:"category"`
	Classification string  `json:"classification"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Evidence       Object  `json:"evidence"`
	Location       Object  `json:"location"`
}
type Report struct {
	Version     string         `json:"aletharsis_version"`
	Schema      string         `json:"schema_version"`
	File        File           `json:"file"`
	Status      string         `json:"status"`
	Evidence    Document       `json:"evidence"`
	Findings    []Finding      `json:"findings"`
	Limitations []string       `json:"limitations"`
	Summary     map[string]int `json:"summary"`
}

func EmptyDocument() Document {
	return Document{Texts: []*Text{}, Metadata: Object{}, Structure: Object{}}
}
func Hash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func Rank(s string) int {
	switch s {
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW", "INFO":
		return 1
	}
	return 0
}
func (r *Report) Summarize() int {
	s := map[string]int{"findings": len(r.Findings), "high": 0, "medium": 0, "low": 0, "info": 0, "exit_code": 0}
	for _, f := range r.Findings {
		switch f.Severity {
		case "HIGH":
			s["high"]++
		case "MEDIUM":
			s["medium"]++
		case "LOW":
			s["low"]++
		case "INFO":
			s["info"]++
		}
		s["exit_code"] = max(s["exit_code"], Rank(f.Severity))
	}
	if r.Status == "failed" {
		s["exit_code"] = 4
	}
	r.Summary = s
	return s["exit_code"]
}
func Location(t *Text, positions []int) Object {
	offsets := make([]int, len(positions))
	for i, p := range positions {
		offsets[i] = t.ByteOffsets[p]
	}
	return Object{"source": t.Source, "character_offsets": positions, "byte_offsets": offsets}
}
