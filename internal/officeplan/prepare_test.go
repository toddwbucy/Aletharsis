package officeplan

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"
)

func TestPrepareRetainsOfficeIdentityAndPartialEvidence(t *testing.T) {
	for _, name := range []string{"office-minimal.docx", "office-odt-minimal.odt", "office-partial-crc.docx"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/" + name)
			if err != nil {
				t.Fatal(err)
			}
			original := bytes.Clone(raw)
			limits := DefaultLimits()
			p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits)
			if err != nil {
				t.Fatal(err)
			}
			if p.Limits != limits || p.Admission.Outcomes.SourceSHA256 != evidence.Hash(raw) {
				t.Fatal("lost configuration or source identity")
			}
			if name == "office-odt-minimal.odt" {
				if p.ODT.Format != "odt" || p.DOCX.Format != "" {
					t.Fatal("wrong ODT identity")
				}
			} else if p.DOCX.Format != "docx" || p.ODT.Format != "" {
				t.Fatal("wrong DOCX identity")
			}
			if name == "office-partial-crc.docx" {
				if p.Admission.Outcomes.State != "partial" {
					t.Fatal("lost CRC outcome")
				}
				bad := 0
				for _, part := range p.Admission.Outcomes.Parts {
					if part.State == "failed" {
						bad++
						if part.Part.SHA256 != "" || len(part.Part.Bytes) != 0 {
							t.Fatal("failed bytes exposed")
						}
					}
				}
				if bad != 1 {
					t.Fatal("wrong failed-part count", bad)
				}
			}
			if !bytes.Equal(raw, original) {
				t.Fatal("source mutated")
			}
		})
	}
}

func TestOfficeLimitsRejectEveryZeroDimension(t *testing.T) {
	// Walk the configuration so newly introduced dimensions cannot silently
	// acquire an unlimited zero-value interpretation.
	var walk func(reflect.Value, []int)
	walk = func(v reflect.Value, path []int) {
		for i := 0; i < v.NumField(); i++ {
			p := append(append([]int{}, path...), i)
			if v.Field(i).Kind() == reflect.Struct {
				walk(v.Field(i), p)
				continue
			}
			l := DefaultLimits()
			reflect.ValueOf(&l).Elem().FieldByIndex(p).SetInt(0)
			if l.Validate() == nil {
				t.Fatalf("zero accepted at %v", p)
			}
			if c, err := l.Catalog("test"); err == nil || c != nil {
				t.Fatal("invalid catalog accepted")
			}
			dst := v4.Package{}
			if l.Record(&dst) == nil || !reflect.DeepEqual(dst, v4.Package{}) {
				t.Fatal("invalid record mutated destination")
			}
		}
	}
	walk(reflect.ValueOf(DefaultLimits()), nil)
	if DefaultLimits().Record(nil) == nil {
		t.Fatal("nil destination accepted")
	}
}

func TestOfficeLimitsRecordedFromConfiguration(t *testing.T) {
	l := DefaultLimits()
	l.Package.SourceBytes = 4096
	l.Package.Entries = 20
	l.Package.PartBytes = 2048
	l.Package.TotalBytes = 8192
	l.ScopeTextUTF8Bytes = 100
	l.ScopeScalarOrigins = 50
	l.Report.InputBytes = 10000
	l.Report.OutputBytes = 9000
	l.Report.Nodes = 1000
	l.Report.Depth = 32
	var p v4.Package
	if err := l.Record(&p); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p.Limits)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]int
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"source_bytes": 4096, "part_count": 20, "part_bytes": 2048, "aggregate_bytes": 8192,
		"max_scope_text_utf8_bytes": 100, "max_scope_scalar_origins": 50, "report_input_bytes": 10000,
		"report_output_bytes": 9000, "report_nodes": 1000, "report_depth": 32}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	c, err := l.Catalog("test")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range c {
		if entry.ID == capability.ParseOfficeID {
			found = true
			if *entry.Limits.InputBytes != 4096 || *entry.Limits.ExpandedBytes != 8192 || *entry.Limits.Objects != 20 {
				t.Fatal(entry.Limits)
			}
		}
	}
	if !found {
		t.Fatal("missing package operation")
	}
}

func TestPrepareRejectsInvalidSourceAndConfiguration(t *testing.T) {
	raw, err := os.ReadFile("../../tests/contracts_v4/fixtures/office-minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		sha    string
		limits Limits
	}{
		{"nil context", nil, evidence.Hash(raw), DefaultLimits()},
		{"wrong identity", context.Background(), evidence.Hash([]byte("other")), DefaultLimits()},
		{"invalid limits", context.Background(), evidence.Hash(raw), Limits{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if p, err := Prepare(tc.ctx, raw, tc.sha, tc.limits); err == nil || p != nil {
				t.Fatal("invalid preparation admitted", p, err)
			}
		})
	}
	limits := DefaultLimits()
	limits.Package.SourceBytes = len(raw) - 1
	if p, err := Prepare(context.Background(), raw, evidence.Hash(raw), limits); err == nil || p != nil {
		t.Fatal("source budget not enforced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p, err := Prepare(ctx, raw, evidence.Hash(raw), DefaultLimits()); err == nil || p != nil {
		t.Fatal("canceled preparation admitted")
	}
}
