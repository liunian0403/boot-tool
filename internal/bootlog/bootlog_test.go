package bootlog

import (
	"errors"
	"testing"
	"time"
)

// t0 is an arbitrary anchor; all offsets are relative to it.
var t0 = time.Date(2026, 11, 8, 12, 12, 20, 38000000, time.UTC)

func ev(provider string, id int, offset time.Duration, data map[string]string) event {
	return event{Provider: provider, ID: id, At: t0.Add(offset), Data: data}
}

func bootMarker(offset time.Duration, bootType string) event {
	return ev(providerKernelBoot, idBootType, offset, map[string]string{"BootType": bootType})
}

func logon(offset time.Duration) event {
	return ev(providerWinlogon, idLogon, offset, nil)
}

// TestAnalyzeHybridBoot is the reported field case: Fast Startup resumed the
// kernel session, so the boot marker is a Kernel-Boot 27 BootType=1 record and
// the duration must be marker→logon (seconds), not the kernel uptime (hours).
func TestAnalyzeHybridBoot(t *testing.T) {
	prev := t0.Add(-time.Hour) // previous cold boot marker
	events := []event{         // newest first, as EvtQuery returns them
		logon(3*time.Second + 123*time.Millisecond),
		bootMarker(0, "1"),
		logon(-time.Hour + 20*time.Second), // that cold boot's logon
		ev(providerKernelGeneral, idSystemStarted, -time.Hour,
			map[string]string{"StartTime": prev.Format(time.RFC3339Nano)}),
	}

	got, err := analyze(events)
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}
	if got.Kind != KindHybrid {
		t.Errorf("kind = %q, want %q", got.Kind, KindHybrid)
	}
	if want := 3*time.Second + 123*time.Millisecond; got.Duration != want {
		t.Errorf("duration = %v, want %v", got.Duration, want)
	}
	if got.Ready.Sub(got.Start) != got.Duration {
		t.Errorf("duration %v != ready-start %v", got.Duration, got.Ready.Sub(got.Start))
	}
	if got.Cold == nil {
		t.Fatal("cold reference = nil, want the previous cold boot")
	}
	if want := 20 * time.Second; got.Cold.Duration != want {
		t.Errorf("cold reference duration = %v, want %v", got.Cold.Duration, want)
	}
	if !got.Cold.At.Equal(prev) {
		t.Errorf("cold reference at = %v, want %v", got.Cold.At, prev)
	}
}

// TestAnalyzeColdBoot covers a full boot: Kernel-General 12 carries StartTime,
// which is more precise than the record timestamp.
func TestAnalyzeColdBoot(t *testing.T) {
	events := []event{
		logon(18 * time.Second),
		ev(providerKernelGeneral, idSystemStarted, 0, map[string]string{
			"StartTime": t0.Format(time.RFC3339Nano),
		}),
	}

	got, err := analyze(events)
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}
	if got.Kind != KindCold {
		t.Errorf("kind = %q, want %q", got.Kind, KindCold)
	}
	if want := 18 * time.Second; got.Duration != want {
		t.Errorf("duration = %v, want %v", got.Duration, want)
	}
	if got.Cold != nil {
		t.Errorf("cold reference = %+v, want nil (no earlier boot in the window)", got.Cold)
	}
}

// TestAnalyzeKeepsFirstLogon: fast user switching or a re-login must not
// shorten the reported boot duration.
func TestAnalyzeKeepsFirstLogon(t *testing.T) {
	events := []event{
		logon(2 * time.Hour), // someone else logged on much later
		logon(15 * time.Second),
		bootMarker(0, "0"),
	}

	got, err := analyze(events)
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}
	if want := 15 * time.Second; got.Duration != want {
		t.Errorf("duration = %v, want %v (the first logon after boot)", got.Duration, want)
	}
}

// TestAnalyzeIgnoresStaleBootMarker: a boot marker is only trusted when a logon
// followed it, otherwise the record must fail loudly and let the caller fall back.
func TestAnalyzeMissingMarkers(t *testing.T) {
	cases := []struct {
		name   string
		events []event
		want   error
	}{
		{
			name:   "no boot marker",
			events: []event{logon(3 * time.Second)},
			want:   ErrNoBootMarker,
		},
		{
			name:   "no logon after boot",
			events: []event{bootMarker(0, "0")},
			want:   ErrNoReadyMarker,
		},
		{
			name:   "empty log window",
			events: nil,
			want:   ErrNoBootMarker,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := analyze(tc.events); !errors.Is(err, tc.want) {
				t.Fatalf("analyze() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestAnalyzeResumeBoot: waking from hibernation produces BootType=2.
func TestAnalyzeResumeBoot(t *testing.T) {
	events := []event{
		logon(5 * time.Second),
		bootMarker(0, "2"),
	}

	got, err := analyze(events)
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}
	if got.Kind != KindResume {
		t.Errorf("kind = %q, want %q", got.Kind, KindResume)
	}
	if want := 5 * time.Second; got.Duration != want {
		t.Errorf("duration = %v, want %v", got.Duration, want)
	}
}
