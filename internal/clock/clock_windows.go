//go:build windows

package clock

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32            = syscall.NewLazyDLL("kernel32.dll")
	procQueryInterruptTime = modkernel32.NewProc("QueryInterruptTime")
)

// newClock returns the platform clock.
func newClock() Clock {
	return windowsClock{}
}

// windowsClock reads the two sources documented in docs/design.md §2.1.
type windowsClock struct{}

// Now returns the wall clock.
func (windowsClock) Now() time.Time { return time.Now() }

// Uptime returns QueryInterruptTime: 100ns units since boot, including time
// spent suspended. Never affected by wall-clock or timezone changes.
func (windowsClock) Uptime() time.Duration {
	var t uint64
	procQueryInterruptTime.Call(uintptr(unsafe.Pointer(&t)))
	// t is in 100ns units; overflow is ~292 years of uptime, irrelevant.
	return time.Duration(t * 100)
}
