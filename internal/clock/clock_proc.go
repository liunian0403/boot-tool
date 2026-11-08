//go:build !windows

package clock

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// newClock returns the platform clock. Non-Windows builds exist for
// development and CI smoke testing only (see docs/design.md §3.2).
func newClock() Clock {
	return procClock{}
}

// procClock reads /proc/uptime (dev convenience; delivery target is Windows).
type procClock struct{}

// Now returns the wall clock.
func (procClock) Now() time.Time { return time.Now() }

// Uptime parses the first field of /proc/uptime, seconds since boot.
func (procClock) Uptime() time.Duration {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0
	}
	sec, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}
