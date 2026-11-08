// Package clock abstracts the two time sources the tool needs: wall clock
// and monotonic uptime including sleep/hibernate time.
package clock

import (
	"sync"
	"time"
)

// Clock provides the two readings every computation in this tool is based on.
//
// Contract:
//   - Now is the wall clock, used only for display and for deriving the
//     absolute boot instant.
//   - Uptime is monotonic time since boot and MUST include sleep/hibernate
//     time (the "正常运行时间" / Task Manager uptime semantics). It is never
//     affected by wall-clock adjustments, timezone or DST changes.
type Clock interface {
	Now() time.Time
	Uptime() time.Duration
}

// Fake is a programmable Clock for tests.
type Fake struct {
	mu sync.Mutex
	up time.Duration
	at time.Time
}

// NewFake returns a Fake at wall time `at` with uptime `up`.
func NewFake(at time.Time, up time.Duration) *Fake {
	return &Fake{at: at, up: up}
}

// Now implements Clock.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.at
}

// Uptime implements Clock.
func (f *Fake) Uptime() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.up
}

// Advance moves wall clock and uptime forward by d, i.e. the machine keeps
// running normally (or sleeps: interrupt time advances during suspend too).
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.at = f.at.Add(d)
	f.up += d
}

// StepWall moves only the wall clock by d (NTP correction / manual clock
// change). Uptime must be unaffected.
func (f *Fake) StepWall(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.at = f.at.Add(d)
}
