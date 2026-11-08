// Package bootlog derives "how long this boot took" from Windows' own boot
// markers in the System event log (docs/design.md §16).
//
// Why not the kernel uptime counter: with Fast Startup (默认开启的"快速启动")
// a shutdown is a hibernate, so the kernel session — and therefore
// QueryInterruptTime / GetTickCount64 / LastBootUpTime — keeps counting across
// the power-off. Measured on the reporting machine: a hybrid boot at 12:12:20
// reported 1h53m of "uptime" because the counter had been running since the
// previous cold boot at 10:21.
//
// The log, however, records every boot separately:
//
//	Microsoft-Windows-Kernel-General 12  - OS started (cold boot)
//	Microsoft-Windows-Kernel-Boot    27  - boot type (0 cold, 1 hybrid, 2 resume)
//	Microsoft-Windows-Winlogon     7001  - user logon completed (desktop ready)
//
// Boot duration = logon(7001) - boot marker(12|27). The timings match Windows'
// own Diagnostics-Performance figures, and unlike that channel the System log is
// readable without administrator rights (the product must stay admin-free).
package bootlog

import (
	"errors"
	"fmt"
	"time"
)

// Kind describes how the OS came up.
type Kind string

const (
	// KindCold is a full boot: firmware → kernel → logon.
	KindCold Kind = "cold"
	// KindHybrid is Fast Startup: the kernel session was hibernated and resumed,
	// which is why the kernel uptime counter does not restart.
	KindHybrid Kind = "hybrid"
	// KindResume is a resume from hibernation (no fresh kernel session at all).
	KindResume Kind = "resume"
	// KindUnknown is a boot marker whose type could not be read.
	KindUnknown Kind = "unknown"
)

// Record is one boot session as reported by the System event log.
type Record struct {
	Start    time.Time     // boot marker (kernel start for cold boots, resume start for hybrid)
	Ready    time.Time     // first logon completed after Start
	Duration time.Duration // Ready - Start = 本次开机耗时
	Kind     Kind
	Source   string // where the record came from, for the startup log

	// Approximate marks a record derived from the kernel uptime counter because
	// the event log could not be read. Under Fast Startup such a value can be
	// hours too large, so the UI must label it instead of trusting it.
	Approximate bool
}

// FromUptime builds the best-effort fallback record when the System log is
// unavailable: the kernel counter is all that is left, and it is only a lower
// bound on how long the machine has been up since the previous cold boot.
func FromUptime(now time.Time, uptime time.Duration) Report {
	if uptime < 0 {
		uptime = 0
	}
	start := now.Add(-uptime)
	return Report{Record: Record{
		Start:       start,
		Ready:       start,
		Duration:    0,
		Kind:        KindUnknown,
		Source:      "kernel-uptime",
		Approximate: true,
	}}
}

// Cold is a earlier cold boot, kept as a reference: a hybrid boot finishes in
// seconds, so showing the last cold boot's duration explains the small number.
type Cold struct {
	Duration time.Duration
	At       time.Time
}

// Report is everything the UI needs about this boot session.
type Report struct {
	Record
	Cold *Cold // nil when no earlier cold boot is in the log window
}

// Errors reported to callers so the app can fall back to the kernel counter
// with a visible note instead of showing a made-up number.
var (
	// ErrNoBootMarker means the log window held no boot start marker.
	ErrNoBootMarker = errors.New("bootlog: no boot marker in the System log window")
	// ErrNoReadyMarker means a boot was found but no logon followed it.
	ErrNoReadyMarker = errors.New("bootlog: boot found but no logon marker after it")
)

// event is one parsed System log record (newest first in the analysed slice).
type event struct {
	Provider string
	ID       int
	At       time.Time
	Data     map[string]string
}

const (
	providerKernelGeneral = "Microsoft-Windows-Kernel-General"
	providerKernelBoot    = "Microsoft-Windows-Kernel-Boot"
	providerWinlogon      = "Microsoft-Windows-Winlogon"

	idSystemStarted = 12 // Kernel-General: OS started
	idBootType      = 27 // Kernel-Boot: boot type
	idLogon         = 7001
)

// analyze turns a newest-first event slice into a Report.
//
// It walks down from the newest record to the first boot marker, which is the
// current session (a machine can hold many boots in one log window), then walks
// back up to the first logon that followed it — later logons are fast user
// switching or re-logins and must not shorten the boot duration.
func analyze(events []event) (Report, error) {
	idx := -1
	for i, e := range events {
		if isBootMarker(e) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Report{}, ErrNoBootMarker
	}

	marker := events[idx]
	rec := Record{
		Start:  bootStart(marker),
		Kind:   bootKind(marker),
		Source: "eventlog:System",
	}

	var ready time.Time
	for i := idx - 1; i >= 0; i-- { // newer events; going backwards keeps the earliest
		e := events[i]
		if !isLogon(e) || e.At.Before(rec.Start) {
			continue
		}
		if e.At.Sub(rec.Start) > time.Hour { // a logon an hour later is not "boot ready"
			continue
		}
		ready = e.At
	}
	if ready.IsZero() {
		return Report{}, ErrNoReadyMarker
	}
	rec.Ready = ready
	rec.Duration = ready.Sub(rec.Start)
	if rec.Duration < 0 {
		return Report{}, fmt.Errorf("bootlog: negative boot duration %v", rec.Duration)
	}

	report := Report{Record: rec}
	if cold, ok := earlierCold(events[idx+1:], rec.Start); ok {
		report.Cold = &cold
	}
	return report, nil
}

// earlierCold finds the most recent cold boot before the current one, for the
// "上次冷启动 13 秒" reference line. Best-effort: a missing reference is fine.
func earlierCold(older []event, before time.Time) (Cold, bool) {
	for i, e := range older {
		if !isBootMarker(e) || bootKind(e) == KindHybrid || bootKind(e) == KindResume {
			continue
		}
		start := bootStart(e)
		if !start.Before(before) {
			continue
		}
		var ready time.Time
		for j := i - 1; j >= 0; j-- {
			cand := older[j]
			if !isLogon(cand) || cand.At.Before(start) || cand.At.Sub(start) > time.Hour {
				continue
			}
			ready = cand.At
		}
		if ready.IsZero() {
			continue
		}
		return Cold{Duration: ready.Sub(start), At: start}, true
	}
	return Cold{}, false
}

func isBootMarker(e event) bool {
	return (e.Provider == providerKernelGeneral && e.ID == idSystemStarted) ||
		(e.Provider == providerKernelBoot && e.ID == idBootType)
}

func isLogon(e event) bool {
	return e.Provider == providerWinlogon && e.ID == idLogon
}

// bootStart uses the kernel-reported start time when the record carries one
// (Kernel-General 12 does, Kernel-Boot 27 does not), falling back to the event
// timestamp — the difference is a few hundred milliseconds.
func bootStart(e event) time.Time {
	if v, ok := e.Data["StartTime"]; ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t
		}
	}
	return e.At
}

func bootKind(e event) Kind {
	if e.Provider == providerKernelGeneral {
		return KindCold
	}
	switch e.Data["BootType"] {
	case "0":
		return KindCold
	case "1":
		return KindHybrid
	case "2":
		return KindResume
	}
	return KindUnknown
}
