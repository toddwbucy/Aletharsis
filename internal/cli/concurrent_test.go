package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

func TestConcurrentIndependentInputsAndReports(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only verified acquisition")
	}
	type job struct {
		source, output string
		data           []byte
		before         os.FileInfo
		code           int
	}
	jobs := make([]job, 16)
	dir := t.TempDir()
	for i := range jobs {
		data := []byte(fmt.Sprintf("input %d: é😀\u200b\n", i))
		code := 1
		if i%5 == 0 {
			data = []byte{0xff}
			code = 4
		}
		source := filepath.Join(dir, fmt.Sprintf("source-%d.txt", i))
		if err := os.WriteFile(source, data, 0600); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(source)
		if err != nil {
			t.Fatal(err)
		}
		jobs[i] = job{source, filepath.Join(dir, fmt.Sprintf("report-%d.json", i)), data, before, code}
	}
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"audit", j.source, "--output", j.output}, &stdout, &stderr); code != j.code || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Errorf("%s: code=%d stderr=%s", j.source, code, stderr.String())
				return
			}
			raw, err := os.ReadFile(j.output)
			if err != nil {
				t.Error(err)
				return
			}
			var report evidence.Report
			if err := json.Unmarshal(raw, &report); err != nil {
				t.Error(err)
				return
			}
			wantHash := fmt.Sprintf("%x", sha256.Sum256(j.data))
			if report.File.Path != j.source || report.File.SHA256 == nil || *report.File.SHA256 != wantHash || report.Summary["exit_code"] != j.code {
				t.Errorf("cross-input identity leaked: %s", j.source)
				return
			}
			if j.code == 4 {
				if report.Status != "failed" || len(report.Evidence.Texts) != 0 {
					t.Errorf("partial evidence: %s", j.source)
				}
			} else if report.Status != "completed" || len(report.Evidence.Texts) != 1 || report.Evidence.Texts[0].Text != string(j.data) {
				t.Errorf("cross-input text leaked: %s", j.source)
			}
			// A second audit of the same input must match the independent output file.
			var repeated bytes.Buffer
			if code := Run([]string{"audit", j.source, "--json"}, &repeated, &stderr); code != j.code || !bytes.Equal(raw, repeated.Bytes()) {
				t.Errorf("nondeterministic report: %s", j.source)
			}
			after, err := os.Stat(j.source)
			if err != nil {
				t.Error(err)
				return
			}
			// Sys holds the native stat snapshot, including access/change timestamps.
			if !reflect.DeepEqual(j.before.Sys(), after.Sys()) {
				t.Errorf("source metadata changed: %s", j.source)
			}
			sourceData, err := os.ReadFile(j.source)
			if err != nil || !bytes.Equal(sourceData, j.data) {
				t.Errorf("source bytes changed: %s (%v)", j.source, err)
			}
		}()
	}
	wg.Wait()
}
