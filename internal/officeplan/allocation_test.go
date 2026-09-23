package officeplan

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func TestPrepareAllocationScalesWithAdmittedPayload(t *testing.T) {
	const payloadBytes = 3 * 8 << 20
	payload := strings.Repeat("x", 8<<20)
	raw := analysisFixture(t, func(parts map[string]string) {
		for _, name := range []string{"a", "b", "c"} {
			parts["word/embeddings/"+name+".bin"] = payload
		}
	})
	// Fixture construction is deliberately outside the measurement. This checks
	// cumulative allocations of the complete staged preparation, not peak RSS.
	digest := evidence.Hash(raw)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	prepared, err := Prepare(context.Background(), raw, digest, DefaultLimits())
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	runtime.KeepAlive(prepared)
	if prepared.Identity != IdentityDOCX || prepared.Admission.Outcomes.State != "completed" {
		t.Fatal("fixture was not fully admitted")
	}
	var admitted int
	for _, o := range prepared.Admission.Outcomes.Parts {
		admitted += len(o.Part.Bytes)
	}
	if admitted < payloadBytes {
		t.Fatal("payload missing from measured work")
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("Prepare allocated %d bytes for %d admitted payload bytes", allocated, admitted)
	if allocated > uint64(3*admitted) {
		t.Fatalf("repeated payload copies: %d allocated / %d admitted", allocated, admitted)
	}
}
