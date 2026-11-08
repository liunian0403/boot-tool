//go:build !windows

// Failure-reporting tests live behind !windows: on Windows Alert opens a real
// modal dialog, which would block the test runner (release CI runs tests on
// windows-latest). Windows-side dialog behaviour is covered by manual
// acceptance B0 (docs/test-plan.md).
package diag

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStderr redirects os.Stderr for fn and returns what was written —
// on non-Windows that is where Alert's "dialog" lands.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() {
		os.Stderr = old
	}()
	fn()
	_ = w.Close()
	out, _ := io.ReadAll(r)
	_ = r.Close()
	return string(out)
}

// TestFailStageLogsAndReturns: a failed stage must be logged as
// "stage: <name> FAILED: <err>", shown to the user with the log path, and
// returned so main can exit non-zero (docs/design.md §5.1).
func TestFailStageLogsAndReturns(t *testing.T) {
	path := withTempLog(t)
	var buf bytes.Buffer
	setMirror(&buf)

	var got error
	shown := captureStderr(t, func() { got = FailStage("window", errors.New("boom")) })

	if got == nil {
		t.Fatal("FailStage returned nil, want the original error")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read log: %v", readErr)
	}
	if !strings.Contains(string(data), "stage: window FAILED: boom") {
		t.Fatalf("log = %q, want the FAILED stage line", data)
	}
	if !strings.Contains(buf.String(), "stage: window FAILED: boom") {
		t.Fatalf("mirror = %q, want the FAILED stage line", buf.String())
	}
	if !strings.Contains(shown, path) {
		t.Fatalf("user message = %q, want the log path %q", shown, path)
	}
}

// TestFailStageNilErrorIsNoop: no stage failed, nothing is reported.
func TestFailStageNilErrorIsNoop(t *testing.T) {
	path := withTempLog(t)

	if got := FailStage("clock", nil); got != nil {
		t.Fatalf("FailStage(nil) = %v, want nil", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("log created for a nil error: err=%v", err)
	}
}

// TestAlertAndFatalNeverPanic: the two user-facing reporting paths must be
// callable at any time, including from a panic handler.
func TestAlertAndFatalNeverPanic(t *testing.T) {
	withTempLog(t)

	captureStderr(t, func() {
		Alert("test alert")
		Fatal(errors.New("test panic"))
	})

	data, err := os.ReadFile(Path())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "test panic") {
		t.Fatalf("log = %q, want the panic record", data)
	}
}
