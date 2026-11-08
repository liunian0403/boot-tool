//go:build windows

package clock_test

import (
	"syscall"
	"testing"
	"time"

	"github.com/liunian0403/boot-tool/internal/clock"
)

// TestWindowsClockResolves is the HYPE-46 regression test: the clock must
// resolve on every supported Windows build, including builds whose kernel32.dll
// no longer forwards QueryInterruptTime (Windows 11 10.0.26300 was the field
// failure). The old code returned "Failed to find QueryInterruptTime procedure
// in kernel32.dll" and the app could not start.
func TestWindowsClockResolves(t *testing.T) {
	c, err := clock.New()
	if err != nil {
		t.Fatalf("clock.New() failed: %v", err)
	}

	src := clock.SourceName(c)
	if src == "" || src == "unknown" {
		t.Errorf("SourceName() = %q, want a named uptime source", src)
	}
	t.Logf("time source: %s", src)

	up := c.Uptime()
	if up <= 0 {
		t.Fatalf("uptime = %v, want > 0", up)
	}
	if up > 100*365*24*time.Hour {
		t.Fatalf("implausible uptime %v", up)
	}

	// The wall clock must agree with the standard library.
	if d := time.Since(c.Now()); d > 5*time.Second || d < -5*time.Second {
		t.Fatalf("Now() off from time.Now by %v", d)
	}

	// Independent cross-check: GetTickCount64 is always present and shares the
	// counter family, so a mismatch means the resolved source is broken.
	tick := syscall.NewLazyDLL("kernel32.dll").NewProc("GetTickCount64")
	if err := tick.Find(); err != nil {
		t.Skipf("GetTickCount64 unavailable: %v", err)
	}
	r, _, _ := tick.Call()
	diff := time.Duration(r)*time.Millisecond - c.Uptime()
	if diff > 2*time.Second || diff < -2*time.Second {
		t.Fatalf("uptime %v differs from GetTickCount64 by %v", c.Uptime(), diff)
	}
}
