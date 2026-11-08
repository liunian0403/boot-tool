// Package gui renders the app view in native Win32 windows: the main window for
// a manual launch and the bottom-right startup popup for an automatic start.
// Uses only syscall-level user32/gdi32 calls so the binary stays
// dependency-free and cross-compiles from any OS (docs/design.md §3.1).
package gui

import "time"

// Mode selects what this process shows (docs/design.md §15.2).
type Mode int

const (
	// ModeWindow is a manual launch: the main window with live uptime.
	ModeWindow Mode = iota
	// ModeToast is the sign-in autostart: a bottom-right popup that reports the
	// boot duration and closes itself. Clicking it opens the main window.
	ModeToast
)

// Options are the run-time knobs of one GUI session.
type Options struct {
	Mode         Mode
	ToastDelay   time.Duration // popup mode: desktop settle time before showing
	ToastTimeout time.Duration // popup mode: how long the popup stays up
}

const (
	// defaultToastDelay lets Explorer finish painting the desktop and the taskbar
	// before the popup appears, so it is not covered by other sign-in windows.
	defaultToastDelay = 3 * time.Second
	// defaultToastTimeout is long enough to read, short enough to stay out of the way.
	defaultToastTimeout = 10 * time.Second
)
