//go:build windows

package bootlog_test

import (
	"testing"
	"time"

	"github.com/liunian0403/boot-tool/internal/bootlog"
)

// TestReadRealBootRecord is the regression test for the field report "刚开机
// 却显示一个多小时": it reads this machine's real System log and requires a
// plausible, self-consistent boot record. Under Fast Startup the kernel counter
// keeps running across the power-off, so this is the only trustworthy source.
func TestReadRealBootRecord(t *testing.T) {
	rep, err := bootlog.Read()
	if err != nil {
		t.Fatalf("bootlog.Read() failed: %v", err)
	}
	t.Logf("kind=%s start=%s ready=%s duration=%s source=%s cold=%v",
		rep.Kind, rep.Start.Format(time.RFC3339), rep.Ready.Format(time.RFC3339),
		rep.Duration.Round(time.Millisecond), rep.Source, rep.Cold)

	if rep.Kind == bootlog.KindUnknown {
		t.Errorf("kind = %q, want a recognised boot type", rep.Kind)
	}
	if rep.Duration <= 0 {
		t.Fatalf("duration = %v, want > 0", rep.Duration)
	}
	if rep.Duration > 10*time.Minute {
		t.Fatalf("duration = %v, implausible for a boot", rep.Duration)
	}
	if rep.Ready.Before(rep.Start) {
		t.Fatalf("ready %v before start %v", rep.Ready, rep.Start)
	}
	if rep.Ready.After(time.Now().Add(time.Minute)) {
		t.Fatalf("ready %v is in the future", rep.Ready)
	}
	if rep.Cold != nil && (rep.Cold.Duration <= 0 || rep.Cold.Duration > 10*time.Minute) {
		t.Errorf("cold reference duration = %v, want 0 < d <= 10m", rep.Cold.Duration)
	}
}
