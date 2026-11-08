package tfmt_test

import (
	"testing"
	"time"

	"github.com/liunian0403/boot-tool/internal/locale"
	"github.com/liunian0403/boot-tool/internal/tfmt"
)

func TestUptimeZh(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0秒"},
		{999 * time.Millisecond, "0秒"},
		{42 * time.Second, "42秒"},
		{61 * time.Second, "1分1秒"},
		{4*time.Hour + 5*time.Minute + 6*time.Second, "4小时5分6秒"},
		{3*24*time.Hour + 4*time.Hour, "3天4小时"},
		{3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second, "3天4小时5分6秒"},
		// Interior zero unit is kept for clarity, trailing zeros dropped.
		{72*time.Hour + 4*time.Minute + 11*time.Second, "3天0小时4分11秒"},
		{-time.Second, "0秒"},
	}
	for _, c := range cases {
		if got := tfmt.Uptime(c.in, locale.LangZh); got != c.want {
			t.Errorf("Uptime(%v, zh) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUptimeEn(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{42 * time.Second, "42s"},
		{61 * time.Second, "1m 1s"},
		{4*time.Hour + 5*time.Minute + 6*time.Second, "4h 5m 6s"},
		{3*24*time.Hour + 4*time.Hour, "3d 4h"},
		{3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second, "3d 4h 5m 6s"},
		{72*time.Hour + 4*time.Minute + 11*time.Second, "3d 0h 4m 11s"},
	}
	for _, c := range cases {
		if got := tfmt.Uptime(c.in, locale.LangEn); got != c.want {
			t.Errorf("Uptime(%v, en) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestBootAcrossZones: formatting follows the instant's location; the same
// instant renders the correct wall time in each zone (AC5 display side).
func TestBootAcrossZones(t *testing.T) {
	instant := time.Date(2026, 11, 8, 9, 15, 32, 0, time.UTC)

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	if got, want := tfmt.Boot(instant.In(shanghai)), "2026-11-08 17:15:32"; got != want {
		t.Errorf("shanghai boot = %q, want %q", got, want)
	}
	// 2026-11-08 is after US DST end (Nov 1): UTC-5 → EST.
	if got, want := tfmt.Boot(instant.In(ny)), "2026-11-08 04:15:32"; got != want {
		t.Errorf("new york boot = %q, want %q", got, want)
	}
}
