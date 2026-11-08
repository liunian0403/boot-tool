package diag

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempLog points the package at a private log file and silences the
// stdout mirror for the duration of the test.
func withTempLog(t *testing.T) string {
	t.Helper()
	mu.Lock()
	oldFile, oldMirror := logFile, mirror
	logFile = filepath.Join(t.TempDir(), "boot-tool.log")
	mirror = io.Discard
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		logFile, mirror = oldFile, oldMirror
		mu.Unlock()
	})
	return logFile
}

func setMirror(w io.Writer) {
	mu.Lock()
	mirror = w
	mu.Unlock()
}

// TestLogfWritesFileAndMirror: one call = one timestamped line on disk and
// the same line mirrored to stdout (that mirror is what boot-tool-debug.exe
// prints).
func TestLogfWritesFileAndMirror(t *testing.T) {
	path := withTempLog(t)
	var buf bytes.Buffer
	setMirror(&buf)

	Logf("hello %d", 42)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "hello 42") {
		t.Fatalf("log = %q, want a line containing \"hello 42\"", data)
	}
	if !strings.Contains(buf.String(), "hello 42") {
		t.Fatalf("mirror = %q, want the same line", buf.String())
	}
}

// TestPathIsReported: users are told where the log lives.
func TestPathIsReported(t *testing.T) {
	path := withTempLog(t)
	if got := Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
}

// TestLogfNeverPanicsWhenLogUnwritable: diagnostics must not break the app —
// an unwritable log path degrades to the mirror only (docs/design.md §5.1).
func TestLogfNeverPanicsWhenLogUnwritable(t *testing.T) {
	withTempLog(t)
	var buf bytes.Buffer
	setMirror(&buf)
	mu.Lock()
	logFile = filepath.Join(t.TempDir(), "no-such-dir", "deeper", "boot-tool.log")
	mu.Unlock()

	Logf("still visible %d", 1) // must not panic

	if !strings.Contains(buf.String(), "still visible 1") {
		t.Fatalf("mirror = %q, want the line to survive the log failure", buf.String())
	}
	if _, err := os.Stat(Path()); !os.IsNotExist(err) {
		t.Fatalf("unexpected log file created: err=%v", err)
	}
}

// TestLogfNeverPanicsOnEmptyPath: same contract for a degenerate path.
func TestLogfNeverPanicsOnEmptyPath(t *testing.T) {
	withTempLog(t)
	mu.Lock()
	logFile = ""
	mu.Unlock()

	Logf("no panic here") // must not panic
}

// TestLogRotatesAtCap: past 512 KB the log rolls to boot-tool.log.1 and the
// active file starts over, so %TEMP% cannot grow without bound
// (docs/design.md §5.1 normalisation 1).
func TestLogRotatesAtCap(t *testing.T) {
	path := withTempLog(t)

	Logf("%s", strings.Repeat("x", maxLogBytes+1))

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	if fi.Size() < maxLogBytes {
		t.Fatalf("log size = %d, want >= %d before rotation", fi.Size(), maxLogBytes)
	}

	Logf("after rotation")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rotated log: %v", err)
	}
	if !strings.Contains(string(data), "after rotation") {
		t.Fatalf("active log = %q, want the newest line", data)
	}
	if strings.Contains(string(data), "xxx") {
		t.Fatalf("active log still holds the pre-rotation payload (%d bytes)", len(data))
	}

	old, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("read rotated copy: %v", err)
	}
	if len(old) < maxLogBytes {
		t.Fatalf("rotated copy = %d bytes, want the previous >= %d payload", len(old), maxLogBytes)
	}
}
