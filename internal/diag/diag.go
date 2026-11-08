// Package diag is the single channel for startup diagnostics and
// user-visible failure reporting (docs/design.md §5): every startup stage is
// appended to %TEMP%\boot-tool.log and mirrored to stdout when a console is
// attached, and every failure raises a dialog.
//
// The release binary runs as a GUI subsystem process, where nothing would
// otherwise be visible — a silent exit is a defect (PRD F6 / AC0), so
// diagnostics must never fail and never break the app itself.
package diag

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxLogBytes caps the log size: once reached the file rotates to
// <log>.1 (replacing the previous copy), so repeated launches can never grow
// %TEMP% without bound. Normal runs only log startup/shutdown/errors — the
// per-second refresh never writes — so this is a defensive ceiling
// (docs/design.md §5.1 normalisation 1).
const maxLogBytes = 512 * 1024

// tsLayout is the timestamp prefix of every log line.
const tsLayout = "2006-01-02 15:04:05.000"

var (
	mu sync.Mutex
	// logFile is a var so tests can point it at a temp dir.
	logFile = filepath.Join(os.TempDir(), "boot-tool.log")
	// mirror receives a copy of every line: stdout for the console build
	// (boot-tool-debug.exe prints the stage sequence), discard-able in tests.
	mirror io.Writer = os.Stdout
)

// Path returns the log file location shown to users in error dialogs.
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logFile
}

// Logf appends a timestamped line to the log file and mirrors it to stdout.
// It never panics and never reports failure: diagnostics must not break the
// app (docs/design.md §5.1).
func Logf(format string, args ...any) {
	// Belt and braces: this is the failure-reporting path, so even a
	// pathological format argument must not take the process down.
	defer func() { _ = recover() }()

	line := fmt.Sprintf("%s %s\n", time.Now().Format(tsLayout), fmt.Sprintf(format, args...))

	mu.Lock()
	defer mu.Unlock()

	fmt.Fprint(mirror, line) // no console attached: the write fails, harmlessly

	f, err := openLocked()
	if err != nil {
		return // log unwritable: degrade to the mirror only (Alert never depends on it)
	}
	defer f.Close()
	_, _ = io.WriteString(f, line)
}

// openLocked opens the log for append, rotating first when the size cap is
// reached. The caller holds mu.
func openLocked() (*os.File, error) {
	if fi, err := os.Stat(logFile); err == nil && fi.Size() >= maxLogBytes {
		rotateLocked()
	}
	return os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// rotateLocked moves the current log to <log>.1, replacing the previous
// copy. The caller holds mu; every step is best-effort — a failed rotation
// must only cost disk hygiene, never a log line or the app.
func rotateLocked() {
	_ = os.Remove(logFile + ".1")
	_ = os.Rename(logFile, logFile+".1")
}

// FailStage reports a failed startup stage: it logs "stage: <name> FAILED:
// <err>", raises a user-visible dialog and returns err so the caller can
// exit non-zero. Any failed stage must be loud (docs/design.md §5.1) — a
// silent exit is exactly the gap PRD F6/AC0 closes.
func FailStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	Logf("stage: %s FAILED: %v", stage, err)
	Alert(fmt.Sprintf(
		"启动失败 / Startup failed\r\n\r\n阶段 / Stage: %s\r\n原因 / Reason: %v\r\n\r\n日志 / Log: %s\r\n\r\n请将日志反馈给开发者 / Please send this log to the developer.",
		stage, err, Path()))
	return err
}
