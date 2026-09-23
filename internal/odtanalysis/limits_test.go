package odtanalysis

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/scopelimits"
)

func TestWholeScopeLimitOmission(t *testing.T) {
	payload := strings.Repeat("\u200b\u200c", 24)
	source := []byte(document(`<text:p>` + payload + `</text:p><text:p>z</text:p>`))
	baseline, err := Analyze(context.Background(), source, evidence.Hash(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Scopes) != 2 || patterns(baseline) != 1 {
		t.Fatal("invalid fixture")
	}
	bytes, scalars := len(payload), utf8.RuneCountInString(payload)
	for _, tc := range []struct {
		name      string
		limits    scopelimits.Limits
		dimension string
	}{
		{"exact", scopelimits.Limits{TextUTF8Bytes: bytes, ScalarOrigins: scalars}, ""},
		{"bytes", scopelimits.Limits{TextUTF8Bytes: bytes - 1, ScalarOrigins: scalars}, "max_scope_text_utf8_bytes"},
		{"origins", scopelimits.Limits{TextUTF8Bytes: bytes, ScalarOrigins: scalars - 1}, "max_scope_scalar_origins"},
		{"both", scopelimits.Limits{TextUTF8Bytes: bytes - 1, ScalarOrigins: scalars - 1}, "max_scope_text_utf8_bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AnalyzeWithLimits(context.Background(), source, evidence.Hash(source), tc.limits)
			if err != nil {
				t.Fatal(err)
			}
			if tc.dimension == "" {
				if !reflect.DeepEqual(got, baseline) {
					t.Fatal("at-limit behavior differs")
				}
				return
			}
			if got.State != "partial" || len(got.Omitted) != 1 || len(got.Scopes) != 1 || patterns(got) != 0 {
				t.Fatal("partial scope or findings survived", got)
			}
			if !reflect.DeepEqual(got.Scopes[0], baseline.Scopes[1]) {
				t.Fatal("later scope identity/coordinates changed")
			}
			o := got.Omitted[0]
			if o.LocalID != "odt-scope/0" || o.Dimension != tc.dimension || o.Code != failure.ResourceLimit || o.TextUTF8Bytes != bytes || o.ScalarOrigins != scalars {
				t.Fatal(o)
			}
			for _, origin := range baseline.Scopes[0].Origins {
				if origin.Source.Start < o.Source.Start || origin.Source.End > o.Source.End {
					t.Fatal("unlocated omission")
				}
			}
			if !reflect.DeepEqual(got.Extraction, baseline.Extraction) {
				t.Fatal("omission changed parser evidence")
			}
		})
	}
	for _, limits := range []scopelimits.Limits{{}, {TextUTF8Bytes: 1}, {ScalarOrigins: 1}, {TextUTF8Bytes: -1, ScalarOrigins: 1}} {
		if got, err := AnalyzeWithLimits(context.Background(), source, evidence.Hash(source), limits); err == nil || got != nil {
			t.Fatal("invalid limits accepted")
		}
	}
}

func TestScopeLimitCountsExpandedControls(t *testing.T) {
	for _, control := range []string{`<text:s text:c="3"/>`, `<text:tab/><text:tab/><text:tab/>`, `<text:line-break/><text:line-break/><text:line-break/>`} {
		source := []byte(document(`<text:p>a` + control + `b</text:p><text:p>z</text:p>`))
		got, err := AnalyzeWithLimits(context.Background(), source, evidence.Hash(source), scopelimits.Limits{TextUTF8Bytes: 100, ScalarOrigins: 4})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Omitted) != 1 || got.Omitted[0].ScalarOrigins != 5 || got.Omitted[0].TextUTF8Bytes != 5 || len(got.Scopes) != 1 || got.Scopes[0].Text != "z" {
			t.Fatal("control origins escaped cap", got)
		}
	}
}
