package schemas

import _ "embed"

//go:embed corpus-v2.schema.json
var corpus2 string

//go:embed corpus-document-v2.schema.json
var corpusDocument2 string

//go:embed reveal-tree-v2.schema.json
var revealTree2 string

// Envelope returns a copy of a known contract. URLs inside imported content
// cannot select contracts or trigger network access.
func Envelope(name string) ([]byte, bool) {
	switch name {
	case "corpus-v2":
		return []byte(corpus2), true
	case "corpus-document-v2":
		return []byte(corpusDocument2), true
	case "reveal-tree-v2":
		return []byte(revealTree2), true
	default:
		return nil, false
	}
}
