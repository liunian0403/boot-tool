//go:build !windows

package bootlog

import "errors"

// Read is unsupported off Windows: the developer console build has no System
// event log to consult, so the caller falls back to the kernel counter.
func Read() (Report, error) {
	return Report{}, errors.New("bootlog: only supported on Windows")
}
