// Package corpus streams bounded audit outcomes without writing source artifacts.
package corpus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/failure"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
	"github.com/toddwbucy/Aletharsis/internal/workspace"
)

var ErrOutputLimit = errors.New("corpus output limit")
var ErrInvalid = errors.New("invalid corpus options")

// ErrSourceLimit is returned only after an observer has retained a report-only
// outcome for this source. It changes the corpus outcome, not the native report.
var ErrSourceLimit = errors.New("observer source resource limit")

// Observer is a trusted in-process presentation boundary, never imported code.
// Callbacks must not mutate or retain the supplied evidence. Errors stop the run
// without a completion record, except Visit returning exactly ErrSourceLimit after
// retaining a report-only outcome. Wrapped/joined errors remain fatal so a
// joined publication failure cannot be swallowed. Observers own rollback.
type Observer interface {
	Prepare([]workspace.Entry) error
	Visit(Entry, Snapshot) error
}
type Snapshot struct {
	Source []byte
	Native *evidence.Report
}

type Options struct {
	Observer                                  Observer
	Discovery                                 workspace.Options
	Schema                                    string
	InputBytes, AcquisitionBytes, OutputBytes int
}
type Entry struct {
	Type                  string          `json:"type"`
	RelativePath          string          `json:"relative_path"`
	State                 string          `json:"state"`
	Reason                string          `json:"reason"`
	Report                json.RawMessage `json:"report,omitempty"`
	ReportCanonicalSHA256 string          `json:"report_canonical_sha256,omitempty"`
}
type Summary struct {
	Type              string         `json:"type"`
	State             string         `json:"state"`
	Reason            string         `json:"reason"`
	DiscoveryComplete bool           `json:"discovery_complete"`
	Entries           int            `json:"entries"`
	Counts            map[string]int `json:"counts"`
	ExitCode          int            `json:"exit_code"`
}

func DefaultOptions() Options {
	return Options{Discovery: workspace.Options{Recursive: false, MaxEntries: 10000, MaxDepth: 64, PathBytes: 4 << 20}, Schema: "1.0", InputBytes: audit.MaxBytes, AcquisitionBytes: 256 << 20, OutputBytes: 128 << 20}
}
func newSummary() Summary {
	return Summary{Type: "summary", State: "completed", Counts: map[string]int{"no_reported_findings": 0, "requires_review": 0, "unsupported": 0, "failed": 0, "skipped": 0, "canceled": 0}}
}

// Run processes one file at a time under the same pinned root used by discovery.
// A final summary is the completion marker. Writer/budget errors leave an
// incomplete stream and return an error; callers must not treat its prefix as a
// completed scan. It does not render, publish, remediate or call external tools.
func Run(ctx context.Context, path string, options Options, out io.Writer) (Summary, error) {
	return run(ctx, path, options, out, nil)
}

// RunRoot uses the caller-owned pinned source root, without closing it.
func RunRoot(ctx context.Context, path string, root *os.Root, options Options, out io.Writer) (Summary, error) {
	if root == nil {
		return newSummary(), ErrInvalid
	}
	return run(ctx, path, options, out, root)
}
func run(ctx context.Context, path string, options Options, out io.Writer, root *os.Root) (Summary, error) {
	summary := newSummary()
	if ctx == nil || out == nil || path == "" || !utf8.ValidString(path) || (options.Schema != "1.0" && options.Schema != "2.0") || options.InputBytes <= 0 || options.InputBytes > audit.MaxBytes || options.OutputBytes <= 0 || options.AcquisitionBytes <= 1 || options.Discovery.MaxEntries <= 0 || options.Discovery.MaxDepth < 0 || options.Discovery.PathBytes <= 0 {
		return summary, ErrInvalid
	}
	options.AcquisitionBytes = min(options.AcquisitionBytes, 1<<30)
	remainingInput := options.AcquisitionBytes
	stream := streamWriter{out: out, remaining: min(options.OutputBytes, 256<<20)}
	if err := stream.emit(map[string]any{"type": "header", "contract": "aletharsis.corpus/1", "workspace": path, "report_schema": options.Schema, "limits": map[string]int{"file_input_bytes": options.InputBytes, "acquisition_bytes": options.AcquisitionBytes, "output_bytes": min(options.OutputBytes, 256<<20), "concurrency": 1, "entries": min(options.Discovery.MaxEntries, 10000), "depth": min(options.Discovery.MaxDepth, 64), "path_bytes": min(options.Discovery.PathBytes, 4<<20)}}); err != nil {
		return summary, err
	}
	finishFailure := func(err error) (Summary, error) {
		summary.State = "failed"
		summary.ExitCode = 4
		summary.Reason = reason(err)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			summary.State = "canceled"
		}
		return summary, stream.emit(summary)
	}
	if err := ctx.Err(); err != nil {
		return finishFailure(err)
	}
	if root == nil {
		var err error
		root, err = workspace.Open(path)
		if err != nil {
			return finishFailure(err)
		}
		defer root.Close()
	}
	discovered, err := workspace.DiscoverRoot(ctx, root, options.Discovery)
	if err != nil {
		return finishFailure(err)
	}
	summary.DiscoveryComplete = discovered.Complete
	if options.Observer != nil {
		if err := options.Observer.Prepare(append([]workspace.Entry(nil), discovered.Entries...)); err != nil {
			return summary, err
		}
	}
	for _, candidate := range discovered.Entries {
		item := Entry{Type: "entry", RelativePath: candidate.RelativePath, State: candidate.State, Reason: candidate.Reason}
		exit := 0
		var snapshot Snapshot
		if candidate.State == "candidate" {
			if err := ctx.Err(); err != nil {
				item.State = "canceled"
				item.Reason = reason(err)
				exit = 4
			} else if remainingInput <= 1 {
				item.State = "failed"
				item.Reason = "execution.resource_limit"
				exit = 4
			} else {
				attemptLimit := min(options.InputBytes, remainingInput-1)
				var charge int
				var report []byte
				var auditFailure *failure.Error
				var runErr error
				if options.Schema == "1.0" {
					result, raw, consumed := audit.InspectRootCounted(root, candidate.RelativePath, attemptLimit, options.InputBytes)
					charge = consumed
					if options.Observer != nil {
						snapshot = Snapshot{Source: raw, Native: result.Report}
					}
					auditFailure = result.Failure
					exit = result.Report.Summary["exit_code"]
					var text string
					text, runErr = reporters.JSON(result.Report, false)
					report = []byte(text)
				} else {
					settings := audit.DefaultV2Options()
					settings.InputBytes = options.InputBytes
					settings.RetainSnapshot = options.Observer != nil
					result, err, consumed := audit.RunV2RootCounted(ctx, root, candidate.RelativePath, settings, attemptLimit)
					charge = consumed
					runErr = err
					if err == nil {
						report = result.JSON
						snapshot = Snapshot{Source: result.Source, Native: result.Native}
						auditFailure = result.Failure
						exit = result.Report.Summary["exit_code"]
					}
				}
				remainingInput -= charge
				item.State = "no_reported_findings"
				if exit > 0 {
					item.State = "requires_review"
				}
				if auditFailure != nil {
					item.State = "failed"
					item.Reason = string(auditFailure.Code())
					if auditFailure.Code() == failure.UnsupportedFormat {
						item.State = "unsupported"
					}
				}
				if exit == 4 && auditFailure == nil {
					item.State = "failed"
					item.Reason = "execution.failed"
					if ctx.Err() != nil {
						item.State = "canceled"
						item.Reason = reason(ctx.Err())
					}
				}
				if runErr == nil {
					report, runErr = identity.Canonicalize(report, audit.DefaultV2Options().ReportLimits)
				}
				if runErr != nil {
					item.State = "failed"
					item.Reason = "execution.failed"
					if errors.Is(runErr, identity.ErrLimit) {
						item.Reason = "execution.report_limit"
					}
					if ctx.Err() != nil {
						item.State = "canceled"
						item.Reason = reason(ctx.Err())
					}
					exit = 4
				} else {
					item.Report = report
					item.ReportCanonicalSHA256 = evidence.Hash(report)
				}
			}
		}
		if item.State == "failed" || item.State == "unsupported" || item.State == "canceled" {
			exit = 4
		}
		if options.Observer != nil {
			if item.State != "no_reported_findings" && item.State != "requires_review" {
				snapshot = Snapshot{}
			}
			observed := item
			observed.Report = bytes.Clone(item.Report)
			if err := options.Observer.Visit(observed, snapshot); err != nil {
				if err == ErrSourceLimit && (item.State == "no_reported_findings" || item.State == "requires_review") {
					item.State = "failed"
					item.Reason = "execution.resource_limit"
					exit = 4
				} else {
					return summary, err
				}
			}
		}
		if err := stream.emit(item); err != nil {
			return summary, err
		}
		summary.Entries++
		summary.Counts[item.State]++
		summary.ExitCode = max(summary.ExitCode, exit)
	}
	if !summary.DiscoveryComplete || summary.Counts["failed"] > 0 || summary.Counts["unsupported"] > 0 {
		summary.State = "partial"
	}
	if ctx.Err() != nil {
		summary.State = "canceled"
		summary.ExitCode = 4
		summary.Reason = reason(ctx.Err())
	}
	return summary, stream.emit(summary)
}
func reason(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "execution.canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "execution.timeout"
	case errors.Is(err, workspace.ErrLimit), errors.Is(err, identity.ErrLimit):
		return "execution.resource_limit"
	case errors.Is(err, workspace.ErrUnavailable):
		return "integrity.no_atime_unavailable"
	case errors.Is(err, workspace.ErrChanged):
		return "file.changed_during_read"
	case errors.Is(err, workspace.ErrInvalid):
		return "execution.unsupported_input"
	default:
		return string(failure.AcquisitionError(err).Code())
	}
}

type streamWriter struct {
	out       io.Writer
	remaining int
}

func (s *streamWriter) emit(value any) error {
	raw, err := reporters.JSON(value, false)
	if err != nil {
		return err
	}
	if len(raw) > s.remaining {
		return ErrOutputLimit
	}
	n, err := s.out.Write([]byte(raw))
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return err
	}
	s.remaining -= n
	return nil
}
