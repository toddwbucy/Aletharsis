package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/corpus"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
	"github.com/toddwbucy/Aletharsis/internal/workspace"
)

func runDirectory(path, output, schema string, recursive, jsonOutput, jsonl, verbose bool, out, errout io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	options := corpus.DefaultOptions()
	options.Schema = schema
	options.Discovery.Recursive = recursive
	mode := "console"
	if jsonOutput || output != "" {
		mode = "json"
	}
	if jsonl {
		mode = "jsonl"
	}
	writer := out
	var sourceRoot, parent *os.Root
	var file *os.File
	var name string
	if output != "" {
		var err error
		sourceRoot, err = workspace.Open(path)
		if err != nil {
			return v2Failure(errout, "input.open_failed", "could not pin source directory")
		}
		defer sourceRoot.Close()
		parent, name, file, err = openCorpusReport(sourceRoot, path, output)
		if err != nil {
			return v2Failure(errout, "output.create_failed", "report must be a new path outside the source tree with real directory ancestors")
		}
		defer parent.Close()
		writer = file
	}
	presentation := &corpusPresentation{out: &boundedCorpusOutput{out: writer, remaining: options.OutputBytes}, mode: mode}
	var summary corpus.Summary
	var err error
	if sourceRoot == nil {
		summary, err = corpus.Run(ctx, path, options, presentation)
	} else {
		summary, err = corpus.RunRoot(ctx, path, sourceRoot, options, presentation)
	}
	if file != nil {
		err = errors.Join(err, file.Close())
		if err != nil {
			err = errors.Join(err, parent.Remove(name))
		}
	}
	if err != nil {
		code := "output.write_failed"
		if errors.Is(err, corpus.ErrOutputLimit) {
			code = "execution.resource_limit"
		}
		return v2Failure(errout, code, "corpus stream did not complete")
	}
	if verbose {
		raw, e := json.Marshal(map[string]any{"event": "corpus_completed", "state": summary.State, "entries": summary.Entries, "exit_code": summary.ExitCode})
		if e != nil || writeComplete(errout, string(raw)+"\n") != nil {
			return 4
		}
	}
	return summary.ExitCode
}

// openCorpusReport checks canonical and inode ancestry before exclusive creation.
// All output access stays relative to a pinned parent. Caller-controlled namespace
// remains a precondition; this is not isolation from privileged mount changes.
func openCorpusReport(source *os.Root, sourcePath, output string) (*os.Root, string, *os.File, error) {
	canonical, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return nil, "", nil, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, "", nil, err
	}
	absolute, err := filepath.Abs(output)
	if err != nil {
		return nil, "", nil, err
	}
	relative, err := filepath.Rel(canonical, absolute)
	if err != nil {
		return nil, "", nil, err
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, "", nil, errors.New("output overlaps source")
	}
	sourceInfo, err := source.Stat(".")
	if err != nil {
		return nil, "", nil, err
	}
	parentPath, name := filepath.Dir(absolute), filepath.Base(absolute)
	var expectedParent os.FileInfo
	for current := parentPath; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return nil, "", nil, err
		}
		if current == parentPath {
			expectedParent = info
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || os.SameFile(sourceInfo, info) {
			return nil, "", nil, errors.New("unsafe output ancestry")
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, "", nil, err
	}
	pinned, err := parent.Stat(".")
	if err != nil || !os.SameFile(expectedParent, pinned) || os.SameFile(sourceInfo, pinned) {
		_ = parent.Close()
		return nil, "", nil, errors.New("unsafe pinned output parent")
	}
	file, err := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		_ = parent.Close()
		return nil, "", nil, err
	}
	return parent, name, file, nil
}

// The executor delivers one bounded complete JSONL record per Write. This adapter
// streams JSON or an escaped console view without retaining a corpus of reports.
type corpusPresentation struct {
	out     io.Writer
	mode    string
	entries int
}

func (p *corpusPresentation) Write(raw []byte) (int, error) {
	if p.mode == "jsonl" {
		if err := writeComplete(p.out, string(raw)); err != nil {
			return 0, err
		}
		return len(raw), nil
	}
	var record struct {
		Workspace string `json:"workspace"`
	}
	var tag struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &tag); err != nil {
		return 0, err
	}
	text := ""
	if p.mode == "json" {
		switch tag.Type {
		case "header":
			text = "{\"header\":" + strings.TrimSpace(string(raw)) + ",\"entries\":["
		case "entry":
			if p.entries > 0 {
				text = ","
			}
			text += strings.TrimSpace(string(raw))
			p.entries++
		case "summary":
			text = "],\"summary\":" + strings.TrimSpace(string(raw)) + "}\n"
		default:
			return 0, errors.New("unknown corpus record")
		}
	} else {
		switch tag.Type {
		case "header":
			if err := json.Unmarshal(raw, &record); err != nil {
				return 0, err
			}
			text = "ALETHARSIS CORPUS AUDIT\nWorkspace: " + u.Escaped(record.Workspace) + "\n"
		case "entry":
			var entry corpus.Entry
			if err := json.Unmarshal(raw, &entry); err != nil {
				return 0, err
			}
			text = "[" + entry.State + "] " + u.Escaped(entry.RelativePath)
			if entry.Reason != "" {
				text += " (" + u.Escaped(entry.Reason) + ")"
			}
			text += "\n"
		case "summary":
			var summary corpus.Summary
			if err := json.Unmarshal(raw, &summary); err != nil {
				return 0, err
			}
			text = fmt.Sprintf("\nSummary: %s; entries=%d; exit=%d; discovery_complete=%t\n", summary.State, summary.Entries, summary.ExitCode, summary.DiscoveryComplete)
			for _, state := range []string{"no_reported_findings", "requires_review", "unsupported", "failed", "skipped", "canceled"} {
				text += fmt.Sprintf("  %s: %d\n", state, summary.Counts[state])
			}
			if summary.Reason != "" {
				text += "Reason: " + u.Escaped(summary.Reason) + "\n"
			}
		default:
			return 0, errors.New("unknown corpus record")
		}
	}
	if err := writeComplete(p.out, text); err != nil {
		return 0, err
	}
	return len(raw), nil
}

// Bound the bytes actually delivered, including JSON-wrapper and console escapes,
// independently of the executor's JSONL serialization budget.
type boundedCorpusOutput struct {
	out       io.Writer
	remaining int
}

func (w *boundedCorpusOutput) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, corpus.ErrOutputLimit
	}
	n, err := w.out.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if n >= 0 && n <= len(p) {
		w.remaining -= n
	}
	return n, err
}
