package v4

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/odtanalysis"
	"github.com/toddwbucy/Aletharsis/internal/wordanalysis"
)

// Contract fixtures are independently built in Python. Check their retained
// coordinates against real format extraction, rather than just self-consistency.
func TestOfficeFixturesAgreeWithNativeExtraction(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts_v4/fixtures/office-*.json")
	if err != nil || len(paths) != 3 {
		t.Fatal("DOCX and ODT fixtures required", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := DecodeReport(raw, identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64})
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(filepath.Join(filepath.Dir(path), r.File.Filename))
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			scope := r.Evidence.Office.Scopes[0]
			var body []byte
			for _, f := range z.File {
				if f.Name != "content.xml" && f.Name != "word/document.xml" {
					continue
				}
				reader, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				body, err = io.ReadAll(io.LimitReader(reader, 1<<20))
				closeErr := reader.Close()
				if err != nil || closeErr != nil {
					t.Fatal(err, closeErr)
				}
			}
			if len(body) == 0 {
				t.Fatal("missing text part")
			}
			switch r.File.Format {
			case "docx":
				actual, err := wordanalysis.Analyze(context.Background(), body, evidence.Hash(body))
				if err != nil {
					t.Fatal(err)
				}
				if len(actual.Scopes) != 1 || actual.Scopes[0].Text != scope.Text || actual.Scopes[0].SHA256 != scope.SHA256 || len(actual.Scopes[0].Origins) != len(scope.Origins) {
					t.Fatal("DOCX scope mismatch")
				}
				for i, origin := range actual.Scopes[0].Origins {
					retained := scope.Origins[i]
					if origin.Source.Start != retained.Source.Start || origin.Source.End != retained.Source.End || origin.UTF8.Start != retained.UTF8.Start || origin.UTF8.End != retained.UTF8.End || retained.Scalar == nil || retained.Segment == nil || origin.Scalar != *retained.Scalar || origin.Segment != *retained.Segment {
						t.Fatal("DOCX origin mismatch", i)
					}
				}
			case "odt":
				actual, err := odtanalysis.Analyze(context.Background(), body, evidence.Hash(body))
				if err != nil {
					t.Fatal(err)
				}
				if len(actual.Scopes) != 1 || actual.Scopes[0].Text != scope.Text || actual.Scopes[0].SHA256 != scope.SHA256 || len(actual.Scopes[0].Origins) != len(scope.Origins) {
					t.Fatal("ODT scope mismatch")
				}
				for i, origin := range actual.Scopes[0].Origins {
					retained := scope.Origins[i]
					if origin.Source.Start != retained.Source.Start || origin.Source.End != retained.Source.End || origin.UTF8.Start != retained.UTF8.Start || origin.UTF8.End != retained.UTF8.End || retained.Scalar == nil || retained.Segment == nil || origin.Scalar != *retained.Scalar || origin.Segment != *retained.Segment {
						t.Fatal("ODT origin mismatch", i)
					}
				}
			default:
				t.Fatal("unexpected Office fixture format")
			}
		})
	}
}
