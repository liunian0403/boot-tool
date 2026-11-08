//go:build windows

package clock

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32   = syscall.NewLazyDLL("kernel32.dll")
	modkernelbase = syscall.NewLazyDLL("kernelbase.dll")
)

// windowsClock reads the two sources documented in docs/design.md §2.1: the
// wall clock, and a resolved monotonic uptime provider.
type windowsClock struct {
	source string
	uptime func() time.Duration
}

// Source names the API that actually answered, for the startup log. Field
// reports are otherwise impossible to correlate with a Windows build.
func (c windowsClock) Source() string { return c.source }

// Now returns the wall clock.
func (windowsClock) Now() time.Time { return time.Now() }

// Uptime returns monotonic time since boot, including sleep/hibernate.
func (c windowsClock) Uptime() time.Duration { return c.uptime() }

// newClock resolves the uptime source, best caliber first (docs/design.md
// §2.1), instead of hard-depending on one export (HYPE-46: kernel32.dll on
// Windows 11 build 26300 has no QueryInterruptTime and the old code aborted):
//
//  1. kernelbase.dll!QueryInterruptTime — the real implementation: 100ns
//     units, includes suspend. Present on every supported Windows 10/11.
//  2. kernel32.dll!QueryInterruptTime — legacy forwarder; absent on some newer
//     builds, kept for older ones where kernelbase may differ.
//  3. kernel32.dll!GetTickCount64 — always present since Vista; 1ms resolution
//     and the same counter family (includes suspend). Last resort, so a
//     missing export can never again make the app unstartable.
func newClock() (Clock, error) {
	kbase := modkernelbase.NewProc("QueryInterruptTime")
	k32 := modkernel32.NewProc("QueryInterruptTime")
	tick := modkernel32.NewProc("GetTickCount64")

	cands := []candidate{
		{name: "kernelbase.dll!QueryInterruptTime", find: kbase.Find},
		{name: "kernel32.dll!QueryInterruptTime", find: k32.Find},
		{name: "kernel32.dll!GetTickCount64", find: tick.Find},
	}
	impls := []func() time.Duration{
		func() time.Duration { return interruptTime(kbase) },
		func() time.Duration { return interruptTime(k32) },
		func() time.Duration { return tickCount64(tick) },
	}

	i, err := choose(cands)
	if err != nil {
		return nil, err
	}
	return windowsClock{source: cands[i].name, uptime: impls[i]}, nil
}

// interruptTime reads QueryInterruptTime/QueryUnbiasedInterruptTime: a 100ns
// counter since boot. Overflow needs ~292 years of uptime, irrelevant here.
func interruptTime(proc *syscall.LazyProc) time.Duration {
	var t uint64
	proc.Call(uintptr(unsafe.Pointer(&t)))
	return time.Duration(t * 100)
}

// tickCount64 reads GetTickCount64: milliseconds since boot, same semantics
// with coarser resolution.
func tickCount64(proc *syscall.LazyProc) time.Duration {
	r, _, _ := proc.Call()
	return time.Duration(r) * time.Millisecond
}
