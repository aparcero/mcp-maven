package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestLogsUseStderr(t *testing.T) {
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalSlog := slog.Default()
	originalDefaultLogger := defaultLogger

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		defaultLogger = originalDefaultLogger
		slog.SetDefault(originalSlog)
		_ = stdoutReader.Close()
		_ = stderrReader.Close()
	}()

	if err := Init("info"); err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	Info("stdio-safe-log")

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("failed to close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("failed to close stderr writer: %v", err)
	}

	stdoutData, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	stderrData, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}

	if string(stdoutData) != "" {
		t.Fatalf("expected logs to avoid stdout, got %q", stdoutData)
	}
	if !strings.Contains(string(stderrData), "stdio-safe-log") {
		t.Fatalf("expected log message on stderr, got %q", stderrData)
	}
}
