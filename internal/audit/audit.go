// Package audit orchestrates read-only snapshot acquisition, parsing and analysis.
package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/parsers"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

const Version = "0.2.0"
const MaxBytes = 8 * 1024 * 1024

var checks = []analyzers.Analyzer{analyzers.Unicode{}, analyzers.Emoji{}, analyzers.Text{}, analyzers.Patterns{}, analyzers.Identifiers{}, analyzers.Metadata{}}

func Run(path string) *evidence.Report { return WithLimit(path, MaxBytes) }
func WithLimit(path string, limit int) *evidence.Report {
	return withReader(path, limit, readSnapshot)
}

func withReader(path string, limit int, read func(string, int) ([]byte, error)) *evidence.Report {
	path = filepath.Clean(path)
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) {
		name = ""
	}
	extension := ""
	if i := strings.LastIndex(name, "."); i > 0 && i < len(name)-1 {
		extension = u.Lower(name[i:])
	}
	r := &evidence.Report{Version: Version, Schema: "1.0", File: evidence.File{Path: path, Filename: name, Extension: extension, MIME: "application/octet-stream", Format: "unknown", Basis: "unavailable"}, Status: "completed", Evidence: evidence.EmptyDocument(), Findings: []evidence.Finding{}, Limitations: analyzers.Limitations()}
	data, err := read(path, limit)
	if err != nil {
		return failed(r, err)
	}
	size, hash := len(data), evidence.Hash(data)
	r.File.Size = &size
	r.File.SHA256 = &hash
	r.File.Format, r.File.MIME, r.File.Basis = parsers.Identify(data, extension)
	if r.File.Format != "text" {
		return failed(r, fmt.Errorf("Unsupported format: %s; M0/M1 support Unicode text source files", r.File.Format))
	}
	parser := parsers.TextParser{}
	parserName := parser.Name()
	r.File.Parser = &parserName
	r.Evidence, err = parser.Parse(data)
	if err != nil {
		return failed(r, err)
	}
	for _, check := range checks {
		r.Findings = append(r.Findings, check.Analyze(&r.Evidence)...)
	}
	sort.SliceStable(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		rank := func(s string) int {
			if s == "INFO" {
				return 0
			}
			return evidence.Rank(s)
		}
		if rank(a.Severity) != rank(b.Severity) {
			return rank(a.Severity) > rank(b.Severity)
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		ak, bk := locationKey(a.Location), locationKey(b.Location)
		if ak != bk {
			return ak < bk
		}
		return a.Title < b.Title
	})
	r.Summarize()
	return r
}
func locationKey(loc evidence.Object) string { b, _ := json.Marshal(loc); return string(b) }
func failed(r *evidence.Report, err error) *evidence.Report {
	r.Status = "failed"
	kind := "ValueError"
	details := evidence.Object{}
	var decode *parsers.DecodeError
	if errors.As(err, &decode) {
		kind = "UnicodeDecodeError"
		details["byte_start"] = decode.Start
		details["byte_end"] = decode.End
		details["reason"] = decode.Reason
	} else if errors.Is(err, os.ErrNotExist) {
		kind = "FileNotFoundError"
	} else if errors.Is(err, os.ErrPermission) {
		kind = "PermissionError"
	} else {
		var pe *os.PathError
		if errors.As(err, &pe) {
			kind = "OSError"
		}
	}
	details["error_type"] = kind
	details["message"] = err.Error()
	r.Findings = append(r.Findings, evidence.Finding{ID: "parser.failure", Severity: "INFO", Confidence: 1, Category: "parser", Classification: "observed_fact", Title: "Audit could not complete", Description: "The file was not fully analyzed. Review the error before interpreting findings.", Evidence: details, Location: evidence.Object{}})
	r.Summarize()
	return r
}
