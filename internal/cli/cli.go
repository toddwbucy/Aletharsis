// Package cli keeps command syntax separate from the audit service boundary.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
)

const help = `aletharsis — read-only text and Unicode forensic auditing

Usage:
  aletharsis audit FILE [--json] [--verbose] [--output NEW_REPORT.json]
  aletharsis audit DIRECTORY [--recursive] [--json|--jsonl] [--output NEW_REPORT]
  aletharsis audit DIRECTORY [--recursive] --reveal-out NEW_DIRECTORY [--json|--jsonl]
  aletharsis audit FILE --reveal-out NEW_DIRECTORY [--json] [--schema-version 1.0|2.0]
  aletharsis unicode|metadata|structure FILE [--json] [--verbose]
  aletharsis --version
  aletharsis --help

Report schema: --schema-version 1.0|2.0 (default 1.0).
Schema 2.0 includes capability and execution coverage, even with no findings.
Exit codes: 0 no findings; 1 INFO/LOW; 2 MEDIUM; 3 HIGH; 4 failure.
`

func Run(args []string, out, errout io.Writer) int {
	failure := func(message string) int { fmt.Fprintln(errout, "aletharsis: "+u.Escaped(message)); return 4 }
	if len(args) == 0 {
		return failure("a command is required")
	}
	if args[0] == "--version" {
		if _, err := fmt.Fprintln(out, "aletharsis "+audit.Version); err != nil {
			return 4
		}
		return 0
	}
	if args[0] == "--help" || args[0] == "-h" {
		if _, err := fmt.Fprint(out, help); err != nil {
			return 4
		}
		return 0
	}
	command := args[0]
	if command != "audit" && command != "unicode" && command != "metadata" && command != "structure" {
		return failure("unknown command: " + command)
	}
	path, output, revealOutput := "", "", ""
	schemaVersion := "1.0"
	jsonOutput, verbose, endFlags := false, false, false
	jsonl, recursive := false, false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !endFlags {
			switch arg {
			case "--":
				endFlags = true
				continue
			case "--help", "-h":
				if _, err := fmt.Fprint(out, help); err != nil {
					return 4
				}
				return 0
			case "--jsonl":
				jsonl = true
				continue
			case "--recursive":
				recursive = true
				continue
			case "--json":
				jsonOutput = true
				continue
			case "--verbose":
				verbose = true
				continue
			case "--schema-version":
				if i+1 >= len(args) || (args[i+1] != "1.0" && args[i+1] != "2.0") {
					return failure("--schema-version requires 1.0 or 2.0")
				}
				i++
				schemaVersion = args[i]
				continue
			case "--reveal-out":
				if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
					return failure("--reveal-out requires a new directory path")
				}
				i++
				revealOutput = args[i]
				continue
			case "--output":
				if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
					return failure("--output requires a path")
				}
				i++
				output = args[i]
				continue
			}
			if strings.HasPrefix(arg, "--schema-version=") {
				schemaVersion = strings.TrimPrefix(arg, "--schema-version=")
				if schemaVersion != "1.0" && schemaVersion != "2.0" {
					return failure("--schema-version requires 1.0 or 2.0")
				}
				continue
			}
			if strings.HasPrefix(arg, "--reveal-out=") {
				revealOutput = strings.TrimPrefix(arg, "--reveal-out=")
				if revealOutput == "" {
					return failure("--reveal-out requires a new directory path")
				}
				continue
			}
			if strings.HasPrefix(arg, "--output=") {
				output = strings.TrimPrefix(arg, "--output=")
				if output == "" {
					return failure("--output requires a path")
				}
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return failure("unknown option: " + arg)
			}
		}
		if path != "" {
			return failure("only one input file is supported")
		}
		path = arg
	}
	if path == "" {
		return failure("a file is required")
	}
	info, statErr := os.Lstat(path)
	directory := statErr == nil && info.IsDir()
	if directory || jsonl || recursive {
		if command != "audit" {
			return failure("directory options require audit")
		}
		if statErr == nil && !directory {
			return failure("--recursive and --jsonl require a directory")
		}
		if jsonOutput && jsonl {
			return failure("choose --json or --jsonl")
		}
		if revealOutput != "" {
			if output != "" {
				return failure("--reveal-out cannot be combined with --output; the tree includes corpus.jsonl")
			}
			return runDirectoryReveal(path, revealOutput, schemaVersion, recursive, jsonOutput, jsonl, verbose, out, errout)
		}
		return runDirectory(path, output, schemaVersion, recursive, jsonOutput, jsonl, verbose, out, errout)
	}
	if revealOutput != "" {
		if command != "audit" || output != "" {
			return failure("--reveal-out requires audit and cannot be combined with --output; the bundle includes report.json")
		}
		return runReveal(path, revealOutput, schemaVersion, jsonOutput, verbose, out, errout)
	}
	if schemaVersion == "2.0" {
		return runV2(command, path, output, jsonOutput, verbose, out, errout)
	}
	r := audit.Run(path)
	if command != "audit" {
		allowed := map[string]bool{"parser": true}
		switch command {
		case "unicode":
			allowed["unicode"] = true
			allowed["possible_steganography"] = true
		case "metadata":
			allowed["metadata"] = true
			allowed["identifier"] = true
			allowed["provenance"] = true
		case "structure":
			for _, c := range []string{"document_structure", "embedded_content", "hidden_content", "visual_watermark"} {
				allowed[c] = true
			}
		}
		selected := r.Findings[:0]
		for _, f := range r.Findings {
			if allowed[f.Category] {
				selected = append(selected, f)
			}
		}
		r.Findings = selected
		r.Limitations = append(r.Limitations, fmt.Sprintf("This is the %s finding view; summary and exit code apply to this view. Full extracted evidence is retained.", command))
		r.Summarize()
	}
	code := r.Summary["exit_code"]
	if verbose {
		log, _ := reporters.JSON(map[string]any{"event": "audit_completed", "path": path, "status": r.Status, "exit_code": code}, false)
		fmt.Fprint(errout, log)
	}
	rendered := ""
	if jsonOutput || output != "" {
		s, err := reporters.JSON(r, true)
		if err != nil {
			return failure(err.Error())
		}
		rendered = s
	} else {
		rendered = reporters.Console(r, verbose)
	}
	if output != "" {
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return failure("could not create report: " + err.Error())
		}
		_, writeErr := io.WriteString(f, rendered)
		closeErr := f.Close()
		if writeErr != nil {
			_ = os.Remove(output)
			return failure("could not write report: " + writeErr.Error())
		}
		if closeErr != nil {
			_ = os.Remove(output)
			return failure("could not close report: " + closeErr.Error())
		}
	} else {
		if _, err := io.WriteString(out, rendered); err != nil {
			return 4
		}
	}
	return code
}
