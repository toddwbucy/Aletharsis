package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/capability"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	v2 "github.com/toddwbucy/Aletharsis/internal/evidence/v2"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

func testCatalog(t *testing.T) []v2.Capability {
	t.Helper()
	c, err := capability.Native(Version, MaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	// A synthetic available memory reader allows portable coordinator tests. The
	// public RunV2 always uses the actual compiled platform capability.
	for i := range c {
		if c[i].ID == capability.AcquireID {
			c[i].Availability = v2.Availability{State: "available"}
		}
	}
	return c
}
func memoryV2(t *testing.T, source []byte, view string) *V2Output {
	t.Helper()
	options := DefaultV2Options()
	options.View = view
	out, err := runV2WithReader(context.Background(), "fixture.txt", options, func(string, int) ([]byte, error) { return bytes.Clone(source), nil }, testCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestNativeV2FrozenInputParity(t *testing.T) {
	paths, err := filepath.Glob("../../reference/python-behavior/inputs/*")
	if err != nil || len(paths) == 0 {
		t.Fatal(err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			reader := func(string, int) ([]byte, error) { return bytes.Clone(data), nil }
			legacy := withReader(path, MaxBytes, reader)
			out, err := runV2WithReader(context.Background(), path, DefaultV2Options(), reader, testCatalog(t))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(legacy.File, out.Report.File) || !reflect.DeepEqual(legacy.Evidence, out.Report.Evidence) {
				t.Fatal("native evidence changed")
			}
			fs := []evidence.Finding{}
			for _, f := range out.Report.Findings {
				old := f.Finding
				if old.ID == "parser.failure" {
					old.Evidence = mapClone(old.Evidence)
					delete(old.Evidence, "failure_code")
				}
				fs = append(fs, old)
			}
			if !reflect.DeepEqual(fs, legacy.Findings) || !reflect.DeepEqual(out.Report.Summary, legacy.Summary) {
				t.Fatal("legacy findings or exits changed")
			}
			imported, err := v2.DecodeReport(out.JSON, DefaultV2Options().ReportLimits)
			if err != nil {
				t.Fatal(err)
			}
			if imported.Status != out.Report.Status {
				t.Fatal("invalid encoded status")
			}
		})
	}
}
func mapClone(m evidence.Object) evidence.Object {
	out := evidence.Object{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
func TestNativeV2DeclaresUnavailableMechanisms(t *testing.T) {
	for _, source := range []string{"", "ordinary ASCII\n", "a\u200bb"} {
		t.Run(source, func(t *testing.T) {
			out := memoryV2(t, []byte(source), "audit")
			if out.Report.Status != v2.Completed || len(out.Report.Capabilities) != 11 || len(out.Report.Executions) != 11 || len(out.Report.Results) != 6 {
				t.Fatal("incomplete native catalog")
			}
			seen := map[v2.Mechanism]bool{}
			for _, c := range out.Report.Capabilities {
				if c.Mechanism != nil {
					seen[*c.Mechanism] = true
				}
			}
			if len(seen) != 3 {
				t.Fatal("missing mechanism declarations")
			}
			for _, e := range out.Report.Executions {
				if e.CapabilityRef == "anthropic.claude_text_watermark" {
					if e.State != v2.NotRun || *e.ReasonCode != failure.Disabled || len(e.AnalyzedScope) != 0 {
						t.Fatal("invented statistical analysis")
					}
				}
			}
		})
	}
}
func TestNativeV2TypedAcquisitionFailure(t *testing.T) {
	out, err := runV2WithReader(context.Background(), "missing.txt", DefaultV2Options(), func(string, int) ([]byte, error) {
		return nil, &os.PathError{Op: "open", Path: "missing.txt", Err: os.ErrNotExist}
	}, testCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Status != v2.Failed || out.Report.Summary["exit_code"] != 4 || out.Report.Findings[0].Evidence["failure_code"] != "file.not_found" || len(out.Report.Artifacts) != 1 || out.Report.Artifacts[0].SHA256 != nil {
		t.Fatal("failure provenance lost")
	}
	if len(out.Report.Results) != 0 {
		t.Fatal("failed audit invented results")
	}
}
func TestNativeV2DoesNotInvokeUnavailableReader(t *testing.T) {
	c := testCatalog(t)
	for i := range c {
		if c[i].ID == capability.AcquireID {
			reason := failure.NoAtimeUnavailable
			c[i].Availability = v2.Availability{State: "unavailable", ReasonCode: &reason}
		}
	}
	out, err := runV2WithReader(context.Background(), "source.txt", DefaultV2Options(), func(string, int) ([]byte, error) { t.Fatal("unavailable reader invoked"); return nil, nil }, c)
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Status != v2.Failed || len(out.Report.Findings) != 0 || out.Report.Executions[0].State != v2.NotRun {
		t.Fatal("unavailable operation reported as run")
	}
}

type boundaryContext struct {
	context.Context
	calls, stop int
}

func (c *boundaryContext) Err() error {
	c.calls++
	if c.calls >= c.stop {
		return context.Canceled
	}
	return nil
}
func TestNativeV2CooperativeCancellation(t *testing.T) {
	for _, tc := range []struct {
		stop    int
		status  v2.State
		results int
	}{{1, v2.Canceled, 0}, {2, v2.Canceled, 0}, {4, v2.Partial, 1}} {
		ctx := &boundaryContext{Context: context.Background(), stop: tc.stop}
		calls := 0
		out, err := runV2WithReader(ctx, "source.txt", DefaultV2Options(), func(string, int) ([]byte, error) { calls++; return []byte("a\u200bb"), nil }, testCatalog(t))
		if err != nil {
			t.Fatal(err)
		}
		if out.Report.Status != tc.status || len(out.Report.Results) != tc.results || out.Report.Summary["exit_code"] != 4 {
			t.Fatalf("boundary %d: wrong cancellation outcome", tc.stop)
		}
		if tc.stop == 1 && calls != 0 {
			t.Fatal("canceled acquisition ran")
		}
		if len(out.Report.Diagnostics) != 1 || out.Report.Diagnostics[0].Code != failure.Canceled {
			t.Fatal("cancellation diagnostic missing")
		}
		for _, f := range out.Report.Findings {
			if f.ID == "parser.failure" {
				t.Fatal("cancellation became parser failure")
			}
		}
	}
}
func TestNativeV2OrderingIndependentOfCatalogAndCompletionMaps(t *testing.T) {
	options := DefaultV2Options()
	source := []byte("a\u200bb \u202e\u202c 🧪\nAuthor: example\n")
	c := testCatalog(t)
	reader := func(string, int) ([]byte, error) { return bytes.Clone(source), nil }
	baseline, err := runV2WithReader(context.Background(), "source.txt", options, reader, c)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(c)
	repeated, err := runV2WithReader(context.Background(), "source.txt", options, reader, c)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseline.JSON, repeated.JSON) {
		t.Fatal("catalog order changed ordinals")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := runV2WithReader(context.Background(), "source.txt", options, reader, c)
			if err != nil {
				t.Error(err)
				return
			}
			if !bytes.Equal(out.JSON, baseline.JSON) {
				t.Error("concurrent run changed output")
			}
		}()
	}
	wg.Wait()
}
func TestNativeV2FiltersKeepPositiveCoverage(t *testing.T) {
	source := []byte("a\u200bb")
	all := memoryV2(t, source, "audit")
	filtered := memoryV2(t, source, "metadata")
	if len(filtered.Report.Findings) != 0 || filtered.Report.Summary["exit_code"] != 0 || !reflect.DeepEqual(all.Report.Graph, filtered.Report.Graph) {
		t.Fatal("filter changed execution evidence")
	}
	for _, result := range filtered.Report.Results {
		if result.Payload.Operation == capability.UnicodeInventoryID && result.Payload.Outcome != "observations_present" {
			t.Fatal("filtered positive became negative")
		}
	}
}
func TestNativeV2OptionsAndReportLimits(t *testing.T) {
	options := DefaultV2Options()
	options.InputBytes = MaxBytes + 1
	_, err := runV2WithReader(context.Background(), "x", options, func(string, int) ([]byte, error) { t.Fatal("invalid options acquired source"); return nil, nil }, testCatalog(t))
	if err == nil {
		t.Fatal("oversized native policy accepted")
	}
	options = DefaultV2Options()
	options.ReportLimits.InputBytes = 100
	_, err = runV2WithReader(context.Background(), "x", options, func(string, int) ([]byte, error) { return []byte("hello"), nil }, testCatalog(t))
	if !errors.Is(err, identity.ErrLimit) {
		t.Fatal("report budget ignored", err)
	}
}
func TestNativeV2JSONContainsStableReferences(t *testing.T) {
	out := memoryV2(t, []byte("a\u200bb"), "audit")
	var value map[string]any
	if err := json.Unmarshal(out.JSON, &value); err != nil {
		t.Fatal(err)
	}
	if value["schema_version"] != "2.0" {
		t.Fatal("wrong wire version")
	}
	if out.Report.Findings[0].Ref != "finding/0" || len(out.Report.Findings[0].AnchorRefs) != 1 || out.Report.Anchors[0].Ref != "anchor/0" {
		t.Fatal("missing references")
	}
}

func TestNativeFindingsDoNotUseCompletionOrder(t *testing.T) {
	source := []byte("🧪 a\u200bb\u202ec\u202c\nAuthor: example\n12345678-1234-1234-1234-123456789abc\nαlpha\n")
	trace := &nativeTrace{ctx: context.Background(), completed: map[string][]evidence.Finding{}}
	outcome := inspectWithTrace("source.txt", MaxBytes, func(string, int) ([]byte, error) { return bytes.Clone(source), nil }, trace)
	first, err := assembleV2(outcome, trace, testCatalog(t), DefaultV2Options())
	if err != nil {
		t.Fatal(err)
	}
	before, err := first.Encode(DefaultV2Options().ReportLimits)
	if err != nil {
		t.Fatal(err)
	}
	copied := map[string][]evidence.Finding{}
	for id, fs := range trace.completed {
		copied[id] = slices.Clone(fs)
		slices.Reverse(copied[id])
	}
	trace.completed = copied
	second, err := assembleV2(outcome, trace, testCatalog(t), DefaultV2Options())
	if err != nil {
		t.Fatal(err)
	}
	after, err := second.Encode(DefaultV2Options().ReportLimits)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("finding production order changed report identities")
	}
}
func TestRequestedUnavailableCapabilityIsNotClean(t *testing.T) {
	for _, participation := range []v2.Participation{v2.Required, v2.Optional} {
		catalog := testCatalog(t)
		for i := range catalog {
			if catalog[i].ID == "aletharsis.c2pa.verify" {
				catalog[i].Participation = participation
			}
		}
		out, err := runV2WithReader(context.Background(), "clean.txt", DefaultV2Options(), func(string, int) ([]byte, error) { return []byte("ordinary"), nil }, catalog)
		if err != nil {
			t.Fatal(err)
		}
		if out.Report.Status != v2.Partial || out.Report.Summary["exit_code"] != 4 || len(out.Report.Findings) != 0 {
			t.Fatal("unavailable requested verification called clean")
		}
	}
}

type deadlineContext struct{ context.Context }

func (deadlineContext) Err() error { return context.DeadlineExceeded }
func TestNativeDeadlineHasTimeoutCode(t *testing.T) {
	out, err := runV2WithReader(deadlineContext{context.Background()}, "source.txt", DefaultV2Options(), func(string, int) ([]byte, error) { t.Fatal("expired deadline acquired input"); return nil, nil }, testCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	if out.Report.Status != v2.Failed || out.Report.Diagnostics[0].Code != failure.Timeout || len(out.Report.Findings) != 0 {
		t.Fatal("timeout misclassified")
	}
}
func TestNativeFullTokenAnchors(t *testing.T) {
	source := []byte("αlpha\nAuthor: Example\\User\n12345678-1234-1234-1234-123456789abc\n")
	out := memoryV2(t, source, "audit")
	scalars := []rune(out.Report.Evidence.Texts[0].Text)
	seen := map[string]bool{}
	for _, f := range out.Report.Findings {
		if f.ID != "text.mixed_script" && f.ID != "provenance.text_marker" && f.ID != "identifier.uuid" {
			continue
		}
		seen[f.ID] = true
		if len(f.AnchorRefs) != 1 {
			t.Fatal("token anchor missing")
		}
		for _, a := range out.Report.Anchors {
			if a.Ref == f.AnchorRefs[0] {
				l := a.Locator.(v2.TextLocator)
				span := l.Spans[0].Scalar
				selected := string(scalars[span.Start:span.End])
				switch f.ID {
				case "text.mixed_script":
					if selected != "αlpha" {
						t.Fatal(selected)
					}
				case "provenance.text_marker":
					if selected != "Author: Example\\User" {
						t.Fatal(selected)
					}
				case "identifier.uuid":
					if selected != "12345678-1234-1234-1234-123456789abc" {
						t.Fatal(selected)
					}
				}
			}
		}
	}

	if len(seen) != 3 {
		t.Fatal("missing token findings", seen)
	}
}
