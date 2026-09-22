package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/corpus"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/publication"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
	"github.com/toddwbucy/Aletharsis/internal/reveal"
	"github.com/toddwbucy/Aletharsis/internal/workspace"
)

type treeObserver struct {
	root *os.Root
	tree *publication.Tree
}

func (o *treeObserver) Prepare(entries []workspace.Entry) error {
	sources := make([]publication.TreeSource, 0, len(entries))
	for _, entry := range entries {
		sources = append(sources, publication.TreeSource{RelativePath: entry.RelativePath, Candidate: entry.State == "candidate"})
	}
	tree, err := publication.NewTree(o.root, sources, publication.TreeLimits{Sources: 10000, Nodes: 100000, Bytes: 256 << 20})
	o.tree = tree
	return err
}
func (o *treeObserver) Visit(entry corpus.Entry, snapshot corpus.Snapshot) error {
	if o.tree == nil {
		return publication.ErrState
	}
	artifacts := map[string][]byte{}
	sourceHash := ""
	if len(entry.Report) > 0 {
		if evidence.Hash(entry.Report) != entry.ReportCanonicalSHA256 {
			return errors.New("report identity changed")
		}
		var report struct {
			File evidence.File `json:"file"`
		}
		if err := json.Unmarshal(entry.Report, &report); err != nil {
			return err
		}
		if report.File.SHA256 != nil {
			sourceHash = *report.File.SHA256
		}
		artifacts["report"] = entry.Report
	}
	state := entry.State
	if state == "no_reported_findings" || state == "requires_review" {
		native := snapshot.Native
		if native == nil || native.File.SHA256 == nil || *native.File.SHA256 != sourceHash || len(native.Evidence.Texts) != 1 {
			return errors.New("missing verified text snapshot")
		}
		comparison, err := reveal.Compare(snapshot.Source, sourceHash, native.Evidence.Texts[0], native.Findings,
			reveal.Limits{SourceBytes: audit.MaxBytes, OutputBytes: 16 << 20, Occurrences: 100000},
			reveal.DiffLimits{TextBytes: 32 << 20, OutputBytes: 32 << 20, Lines: 200000, Escapes: 100000, Context: 3})
		if err != nil {
			if errors.Is(err, reveal.ErrLimit) || errors.Is(err, reveal.ErrDiffLimit) {
				// Retain this source's audit report and continue the corpus. A
				// publication failure remains fatal and must not be masked.
				if recordErr := o.tree.Record(entry.RelativePath, "failed", "execution.resource_limit", sourceHash, artifacts); recordErr != nil {
					return recordErr
				}
				return corpus.ErrSourceLimit
			}
			return err
		}
		mapping, err := reporters.JSON(comparison, false)
		if err != nil {
			return err
		}
		artifacts["revealed"] = []byte(comparison.Reveal.Text)
		artifacts["mapping"] = []byte(mapping)
		artifacts["faithful"] = []byte(comparison.Faithful.Text)
		artifacts["display"] = []byte(comparison.Presentation.Diff.Text)
		state = "revealed"
	}
	return o.tree.Record(entry.RelativePath, state, entry.Reason, sourceHash, artifacts)
}

func runDirectoryReveal(path, output, schema string, recursive, jsonOutput, jsonl, verbose bool, out, errout io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	source, err := workspace.Open(path)
	if err != nil {
		return v2Failure(errout, "input.open_failed", "could not pin source directory")
	}
	defer source.Close()
	parent, name, err := corpusOutputParent(source, path, output)
	if err != nil {
		return v2Failure(errout, "output.create_failed", "reveal root must be a new path outside the source tree with resolvable directory ancestors")
	}
	defer parent.Close()
	if err := parent.Mkdir(name, 0700); err != nil {
		return v2Failure(errout, "output.create_failed", "reveal destination already exists or cannot be created")
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		_ = parent.Remove(name)
		return v2Failure(errout, "output.create_failed", "could not open new reveal root")
	}
	observer := &treeObserver{root: child}
	cleanup := func() error {
		var cause error
		if observer.tree != nil {
			abort := observer.tree.Abort()
			if !errors.Is(abort, publication.ErrState) {
				cause = errors.Join(cause, abort)
			}
		}
		return errors.Join(cause, child.Close(), parent.Remove(name))
	}
	options := corpus.DefaultOptions()
	options.Schema = schema
	options.Discovery.Recursive = recursive
	options.Observer = observer
	var stream bytes.Buffer
	summary, err := corpus.RunRoot(ctx, path, source, options, &stream)
	fail := func(cause error) int {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return v2Failure(errout, "output.cleanup_failed", "reveal publication failed and cleanup is incomplete")
		}
		if errors.Is(cause, publication.ErrPortableName) {
			return v2Failure(errout, "execution.unsupported_input", "a source name cannot be exported under the portable-name policy")
		}
		code := "output.publish_failed"
		if errors.Is(cause, publication.ErrCollision) {
			code = "output.path_collision"
		}
		if errors.Is(cause, publication.ErrLimit) || errors.Is(cause, reveal.ErrDiffLimit) || errors.Is(cause, corpus.ErrOutputLimit) {
			code = "execution.resource_limit"
		}
		return v2Failure(errout, code, "reveal transaction failed and was rolled back")
	}
	if err != nil {
		return fail(err)
	}
	published := false
	if observer.tree != nil {
		if _, err := observer.tree.Commit(stream.Bytes()); err != nil {
			return fail(err)
		}
		if err := child.Close(); err != nil {
			return v2Failure(errout, "output.close_failed", "reveal tree was published but root close failed")
		}
		published = true
	} else {
		// Discovery failed or was canceled before a plan existed. Retain the audit
		// result on stdout but publish no empty/misleading derivative tree.
		if err := cleanup(); err != nil {
			return v2Failure(errout, "output.cleanup_failed", "could not remove unused reveal directory")
		}
	}
	mode := "console"
	if jsonOutput {
		mode = "json"
	}
	if jsonl {
		mode = "jsonl"
	}
	presentation := &corpusPresentation{out: &boundedCorpusOutput{out: out, remaining: options.OutputBytes}, mode: mode}
	decoder := json.NewDecoder(bytes.NewReader(stream.Bytes()))
	for {
		var raw json.RawMessage
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return v2Failure(errout, "output.write_failed", "corpus replay failed")
		}
		// The saved JSONL stream uses one newline per record; preserve it on stdout.
		raw = append(raw, '\n')
		if _, err := presentation.Write(raw); err != nil {
			return v2Failure(errout, "output.write_failed", "report output failed; any published reveal tree remains")
		}
	}
	if verbose {
		log, err := json.Marshal(map[string]any{"event": "corpus_reveal_completed", "published": published, "state": summary.State, "entries": summary.Entries, "exit_code": summary.ExitCode})
		if err != nil || writeComplete(errout, string(log)+"\n") != nil {
			return 4
		}
	}
	return summary.ExitCode
}
