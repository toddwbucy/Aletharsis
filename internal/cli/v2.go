package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/toddwbucy/Aletharsis/internal/audit"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
)

func v2Failure(errout io.Writer, code, message string) int {
	// Fixed messages avoid incorporating source text, paths or upstream errors.
	fmt.Fprintf(errout, "aletharsis: %s: %s\n", code, message)
	return 4
}

func runV2(command, path, output string, jsonOutput, verbose bool, out, errout io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	options := audit.DefaultV2Options()
	options.View = command
	result, err := audit.RunV2(ctx, path, options)
	if err != nil {
		code := "execution.failed"
		if errors.Is(err, identity.ErrLimit) {
			code = "execution.resource_limit"
		}
		return v2Failure(errout, code, "could not assemble a complete report")
	}
	code := result.Report.Summary["exit_code"]
	if verbose {
		log, err := reporters.JSON(map[string]any{"event": "audit_completed", "schema_version": "2.0", "path": path, "status": result.Report.Status, "exit_code": code}, false)
		if err != nil || writeComplete(errout, log) != nil {
			return 4
		}
	}
	rendered := string(result.JSON)
	if !jsonOutput && output == "" {
		rendered = reporters.ConsoleV2(&result.Report, verbose)
	}
	// Canonical JSON is delivered exactly as validated, without a trailing newline.
	if errCode := deliverV2(rendered, output, out, openReport, os.Remove); errCode != "" {
		return v2Failure(errout, errCode, "report delivery failed")
	}
	return code
}

func openReport(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}
func writeComplete(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

// deliverV2 never removes a pre-existing destination. Cleanup applies only after
// exclusive creation succeeded; an incomplete report must not look authoritative.
func deliverV2(text, path string, out io.Writer, create func(string) (io.WriteCloser, error), remove func(string) error) string {
	if path == "" {
		if writeComplete(out, text) != nil {
			return "output.write_failed"
		}
		return ""
	}
	f, err := create(path)
	if err != nil {
		return "output.create_failed"
	}
	writeErr := writeComplete(f, text)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = remove(path)
		if writeErr != nil {
			return "output.write_failed"
		}
		return "output.close_failed"
	}
	return ""
}
