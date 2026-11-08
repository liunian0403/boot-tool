// Package uptime implements the boot-time semantics defined in docs/PRD.md §2
// and docs/design.md §2: uptime includes sleep, boot instant is derived from
// the monotonic uptime counter, wall clock is display-only.
package uptime

import (
	"time"

	"github.com/liunian0403/boot-tool/internal/clock"
)

// Snapshot is one consistent reading taken from a Clock.
type Snapshot struct {
	// TakenAt is the wall-clock instant the reading was taken.
	TakenAt time.Time
	// Up is time since boot including sleep/hibernate time.
	Up time.Duration
	// Boot is the absolute boot instant: TakenAt - Up.
	Boot time.Time
}

// Read takes a snapshot. Boot is re-derived on every call so that timezone
// changes and clock corrections only affect display, never the uptime counter.
func Read(c clock.Clock) Snapshot {
	now := c.Now()
	up := c.Uptime()
	if up < 0 {
		up = 0 // defensive: a clock must not report negative uptime
	}
	return Snapshot{
		TakenAt: now,
		Up:      up,
		Boot:    now.Add(-up),
	}
}
