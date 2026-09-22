package wordtext

import "github.com/toddwbucy/Aletharsis/internal/xmlparts"

// AnalysisContext is admission to the bounded stored-text grammar, not OOXML
// validity or rendered visibility. BlockedElement is the first unsupported edge
// on the path from the selected story root; -1 means the path is admitted.
type AnalysisContext struct {
	Mode           string
	BlockedElement int
}

func analysisContexts(d *xmlparts.Document, ns string, scope int) []AnalysisContext {
	out := make([]AnalysisContext, len(d.Elements))
	for i, e := range d.Elements {
		out[i] = AnalysisContext{BlockedElement: i}
		if i == scope {
			mode := "block"
			switch e.Name.Local {
			case "comments", "footnotes", "endnotes":
				mode = e.Name.Local
			}
			out[i] = AnalysisContext{mode, -1}
			continue
		}
		if e.Parent < 0 {
			continue
		}
		parent := out[e.Parent]
		if parent.BlockedElement >= 0 {
			out[i].BlockedElement = parent.BlockedElement
			continue
		}
		if e.Name.Namespace != ns {
			continue
		}
		mode := ""
		switch parent.Mode {
		case "comments":
			if e.Name.Local == "comment" {
				mode = "block"
			}
		case "footnotes":
			if e.Name.Local == "footnote" {
				mode = "block"
			}
		case "endnotes":
			if e.Name.Local == "endnote" {
				mode = "block"
			}
		case "table":
			if e.Name.Local == "tr" {
				mode = "row"
			}
		case "row":
			if e.Name.Local == "tc" {
				mode = "block"
			}
		case "sdt-block", "sdt-inline":
			if e.Name.Local == "sdtContent" {
				if parent.Mode == "sdt-block" {
					mode = "block"
				} else {
					mode = "inline"
				}
			}
		case "ruby":
			if e.Name.Local == "rubyBase" || e.Name.Local == "rt" {
				mode = "inline"
			}
		case "run":
			if role(e.Name.Local) != "" || e.Name.Local == "tab" || e.Name.Local == "br" || e.Name.Local == "cr" {
				mode = "leaf"
			}
			if e.Name.Local == "ruby" {
				mode = "ruby"
			}
		case "block", "inline":
			switch e.Name.Local {
			case "ins", "del", "moveFrom", "moveTo", "customXml":
				mode = parent.Mode
			case "sdt":
				mode = "sdt-" + parent.Mode
			}
			if parent.Mode == "block" {
				switch e.Name.Local {
				case "p":
					mode = "inline"
				case "tbl":
					mode = "table"
				}
			} else {
				switch e.Name.Local {
				case "r":
					mode = "run"
				case "hyperlink", "smartTag", "fldSimple", "bdo", "dir":
					mode = "inline"
				}
			}
		}
		if mode != "" {
			out[i] = AnalysisContext{mode, -1}
		}
	}
	return out
}
