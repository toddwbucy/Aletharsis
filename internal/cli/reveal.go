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

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/publication"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
	"github.com/toddwbucy/Aletharsis/internal/reveal"
)

func runReveal(path, output, schema string, jsonOutput, verbose bool, out, errout io.Writer) int {
	var source, report []byte
	var native *evidence.Report
	var console string
	code := 4
	if schema == "2.0" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		options := audit.DefaultV2Options()
		options.RetainSnapshot = true
		result, err := audit.RunV2(ctx, path, options)
		if err != nil {
			return v2Failure(errout, "execution.failed", "could not assemble reveal report")
		}
		source, native, report = result.Source, result.Native, result.JSON
		console = reporters.ConsoleV2(&result.Report, verbose)
		code = result.Report.Summary["exit_code"]
	} else {
		result, data := audit.InspectSnapshot(path, audit.MaxBytes)
		native, source = result.Report, data
		encoded, err := reporters.JSON(native, true)
		if err != nil {
			return v2Failure(errout, "execution.failed", "could not serialize reveal report")
		}
		report = []byte(encoded)
		console = reporters.Console(native, verbose)
		code = native.Summary["exit_code"]
	}
	if verbose {
		log, err := reporters.JSON(map[string]any{"event": "audit_completed", "schema_version": schema, "path": path, "exit_code": code}, false)
		if err != nil || writeComplete(errout, log) != nil {
			return 4
		}
	}
	if code == 4 || native == nil || native.Status != "completed" {
		rendered := console
		if jsonOutput {
			rendered = string(report)
		}
		if writeComplete(out, rendered) != nil {
			return 4
		}
		return 4
	}
	if len(native.Evidence.Texts) != 1 || native.File.SHA256 == nil {
		return v2Failure(errout, "reveal.unsupported", "requires one verified text segment")
	}
	comparison, err := reveal.Compare(source, *native.File.SHA256, native.Evidence.Texts[0], native.Findings,
		reveal.Limits{SourceBytes: audit.MaxBytes, OutputBytes: 16 << 20, Occurrences: 100000},
		reveal.DiffLimits{TextBytes: 32 << 20, OutputBytes: 32 << 20, Lines: 200000, Escapes: 100000, Context: 3})
	if err != nil {
		return v2Failure(errout, "reveal.failed", "could not produce a bounded verified comparison")
	}
	mapping, err := json.Marshal(comparison)
	if err != nil {
		return v2Failure(errout, "reveal.failed", "could not serialize comparison")
	}
	artifacts := []publication.Artifact{
		{Name: "report.json", Data: report},
		{Name: "revealed.txt", Data: []byte(comparison.Reveal.Text)},
		{Name: "comparison.json", Data: append(mapping, '\n')},
		{Name: "faithful.diff", Data: []byte(comparison.Faithful.Text)},
		{Name: "display.diff", Data: []byte(comparison.Presentation.Diff.Text)},
	}
	if err := publishReveal(output, *native.File.SHA256, evidence.Hash(report), artifacts); err != nil {
		return v2Failure(errout, "output.publish_failed", "could not publish new reveal directory; inspect destination for incomplete artifacts")
	}
	rendered := console
	if jsonOutput {
		rendered = string(report)
	}
	if writeComplete(out, rendered) != nil {
		return v2Failure(errout, "output.write_failed", "report output failed; reveal bundle was published")
	}
	if verbose && writeComplete(errout, "{\"event\":\"reveal_published\"}\n") != nil {
		return 4
	}
	return code
}

// Output ancestors must already exist; parent aliases are resolved before pinning.
// A pinned Root contains writes under that parent. The newly created private child must remain
// under caller control: this is not isolation from hostile same-user processes.
func publishReveal(output, sourceHash, reportHash string, artifacts []publication.Artifact) error {
	absolute, err := resolveOutput(output)
	if err != nil {
		return err
	}
	parent, name := filepath.Dir(absolute), filepath.Base(absolute)
	if name == "." || name == string(filepath.Separator) {
		return errors.New("invalid output directory")
	}
	var expectedParent os.FileInfo
	for current := parent; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if current == parent {
			expectedParent = info
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("output ancestor is not a real directory")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	pinned, err := root.Stat(".")
	if err != nil || !os.SameFile(expectedParent, pinned) {
		return errors.New("output parent identity changed")
	}
	if err := root.Mkdir(name, 0700); err != nil {
		return err
	}
	child, err := root.OpenRoot(name)
	if err != nil {
		return errors.Join(err, root.Remove(name))
	}
	_, publishErr := publication.Publish(child, sourceHash, reportHash, artifacts, publication.Limits{Files: 8, Bytes: 64 << 20})
	closeErr := child.Close()
	if publishErr != nil {
		// Remove only an empty failed directory. Never recursively delete artifacts.
		return errors.Join(publishErr, closeErr, root.Remove(name))
	}
	if closeErr != nil {
		return fmt.Errorf("published bundle root close: %w", closeErr)
	}
	return nil
}

// Resolve only existing ancestors, never the final destination. Exclusive
// creation still rejects destination symlinks, files and directories unchanged.
func resolveOutput(output string) (string, error) {
	absolute, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	if filepath.Dir(absolute) == absolute {
		return "", errors.New("invalid output destination")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}
