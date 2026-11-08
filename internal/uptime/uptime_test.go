package uptime_test

import (
	"testing"
	"time"

	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/uptime"
)

// start is an arbitrary reference boot/wall instant (UTC).
var start = time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)

// TestBootInstantStableNoDrift covers PRD AC2: uptime tracking for 7 days
// must not drift — the derived boot instant stays exact.
func TestBootInstantStableNoDrift(t *testing.T) {
	f := clock.NewFake(start, 3*time.Hour)
	wantBoot := start.Add(-3 * time.Hour)

	// 7 days in 1-minute steps: 10080 iterations.
	step := time.Minute
	for i := 0; i < 7*24*60; i++ {
		f.Advance(step)
		s := uptime.Read(f)
		if !s.Boot.Equal(wantBoot) {
			t.Fatalf("iter %d: boot drifted: got %v want %v", i, s.Boot, wantBoot)
		}
		if want := 3*time.Hour + time.Duration(i+1)*step; s.Up != want {
			t.Fatalf("iter %d: uptime got %v want %v", i, s.Up, want)
		}
	}
}

// TestSleepContinuity covers PRD AC3: time spent suspended counts toward
// uptime and never resets or jumps the boot instant.
func TestSleepContinuity(t *testing.T) {
	f := clock.NewFake(start, time.Hour)
	boot := uptime.Read(f).Boot

	f.Advance(4 * time.Hour) // sleep 4h: interrupt time advances during suspend
	s := uptime.Read(f)
	if s.Up != 5*time.Hour {
		t.Fatalf("after 4h sleep: uptime = %v, want 5h", s.Up)
	}
	if !s.Boot.Equal(boot) {
		t.Fatalf("boot instant jumped after sleep: got %v want %v", s.Boot, boot)
	}

	f.Advance(30 * time.Minute) // awake again
	s = uptime.Read(f)
	if s.Up != 5*time.Hour+30*time.Minute {
		t.Fatalf("after wake: uptime = %v, want 5h30m", s.Up)
	}
	if !s.Boot.Equal(boot) {
		t.Fatalf("boot instant jumped after wake: got %v want %v", s.Boot, boot)
	}
}

// TestWallStepDoesNotAffectUptime: NTP corrections move only the wall clock;
// the uptime counter (the number users compare) is untouched.
func TestWallStepDoesNotAffectUptime(t *testing.T) {
	f := clock.NewFake(start, 2*time.Hour)
	before := uptime.Read(f)

	f.StepWall(2 * time.Second) // clock stepped forward by 2s

	after := uptime.Read(f)
	if after.Up != before.Up {
		t.Fatalf("uptime affected by wall step: got %v want %v", after.Up, before.Up)
	}
	if !after.Boot.Equal(before.Boot.Add(2 * time.Second)) {
		t.Fatalf("boot should follow wall step by exactly the step, got %v want %v",
			after.Boot, before.Boot.Add(2*time.Second))
	}
}

// TestTimeZoneChangeDoesNotJump covers PRD AC5: changing the display zone
// changes formatting only, never the underlying instants.
func TestTimeZoneChangeDoesNotJump(t *testing.T) {
	f := clock.NewFake(start, 10*time.Hour)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	s1 := uptime.Read(f)
	s2 := uptime.Read(f)
	if !s1.Boot.Equal(s2.Boot) || s1.Up != s2.Up {
		t.Fatal("readings differ")
	}
	// Same instants, different display zones.
	if s1.Boot.In(shanghai).Format(time.RFC3339) == s1.Boot.In(ny).Format(time.RFC3339) {
		t.Fatal("expected different wall display across zones")
	}
	if !s1.Boot.In(shanghai).Equal(s1.Boot.In(ny)) {
		t.Fatal("zone conversion changed the instant")
	}
}

// TestNegativeUptimeClamped: a clock reporting garbage must not produce a
// boot instant in the future.
func TestNegativeUptimeClamped(t *testing.T) {
	f := clock.NewFake(start, -5*time.Second)
	s := uptime.Read(f)
	if s.Up != 0 {
		t.Fatalf("uptime clamped = %v, want 0", s.Up)
	}
	if !s.Boot.Equal(start) {
		t.Fatalf("boot = %v, want %v", s.Boot, start)
	}
}
