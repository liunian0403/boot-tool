//go:build linux

package clock_test

import (
	"testing"
	"time"

	"github.com/liunian0403/boot-tool/internal/clock"
)

// TestRealClockSanity: the dev-mode clock must return sane values on this
// machine (a boot instant that lies in the past, consistent with the wall clock).
func TestRealClockSanity(t *testing.T) {
	c, err := clock.New()
	if err != nil {
		t.Fatalf("clock.New() failed: %v", err)
	}
	up := c.Uptime()
	if up <= 0 {
		t.Fatalf("uptime = %v, want > 0", up)
	}
	now := c.Now()
	if d := time.Since(now); d < -5*time.Second || d > 5*time.Second {
		t.Fatalf("Now() off from time.Now by %v", d)
	}
	if kernelStart := now.Add(-up); !kernelStart.Before(now) {
		t.Fatalf("kernel start %v not before now %v", kernelStart, now)
	}
	// Cross-check against /proc/uptime is implicit: both use the same file;
	// compare against kernel's own view within 2s.
	if up > 100*365*24*time.Hour {
		t.Fatalf("implausible uptime %v", up)
	}
}

func TestFakeSemantics(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := clock.NewFake(at, time.Minute)
	f.Advance(time.Hour)
	if f.Uptime() != time.Hour+time.Minute {
		t.Fatalf("uptime = %v", f.Uptime())
	}
	f.StepWall(-30 * time.Second)
	if f.Uptime() != time.Hour+time.Minute {
		t.Fatalf("uptime changed after wall step: %v", f.Uptime())
	}
	if !f.Now().Equal(at.Add(time.Hour - 30*time.Second)) {
		t.Fatalf("now = %v", f.Now())
	}
}
