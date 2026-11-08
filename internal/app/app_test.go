package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/bootlog"
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/locale"
)

// report builds a boot record: the machine became ready at `ready` after taking
// `dur`, and the wall clock is now `now`.
func report(now, ready time.Time, dur time.Duration, kind bootlog.Kind) bootlog.Report {
	return bootlog.Report{Record: bootlog.Record{
		Start:    ready.Add(-dur),
		Ready:    ready,
		Duration: dur,
		Kind:     kind,
		Source:   "eventlog:System",
	}}
}

// TestViewZh covers a cold boot: the headline is the boot duration from the
// system log, and uptime counts from "desktop ready" rather than from the
// kernel session (Fast Startup makes those two differ by hours).
func TestViewZh(t *testing.T) {
	ready := time.Date(2026, 11, 8, 10, 22, 9, 0, time.Local)
	now := ready.Add(3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second)
	f := clock.NewFake(now, 72*time.Hour) // kernel uptime is irrelevant to the view

	v := app.New(f, locale.LangZh, report(now, ready, 14*time.Second, bootlog.KindCold)).View()
	if v.Title != "开机小助手" {
		t.Errorf("title = %q", v.Title)
	}
	if want := "已运行：3天4小时5分6秒"; v.Up != want {
		t.Errorf("up = %q, want %q", v.Up, want)
	}
	if want := "本次开机耗时：14秒"; v.BootDur != want {
		t.Errorf("duration = %q, want %q", v.BootDur, want)
	}
	if want := "开机完成：2026-11-08 10:22:09"; v.BootDone != want {
		t.Errorf("ready = %q, want %q", v.BootDone, want)
	}
	if !strings.Contains(v.Hint, "系统启动日志") {
		t.Errorf("hint = %q", v.Hint)
	}
}

// TestViewEn mirrors the assertions for English systems.
func TestViewEn(t *testing.T) {
	ready := time.Date(2026, 11, 8, 10, 22, 9, 0, time.Local)
	now := ready.Add(time.Hour)
	f := clock.NewFake(now, time.Hour)

	v := app.New(f, locale.LangEn, report(now, ready, 29*time.Second, bootlog.KindCold)).View()
	if v.Title != "Boot Assistant" {
		t.Errorf("title = %q", v.Title)
	}
	if want := "Uptime: 1h"; v.Up != want {
		t.Errorf("up = %q, want %q", v.Up, want)
	}
	if want := "Boot took: 29s"; v.BootDur != want {
		t.Errorf("duration = %q, want %q", v.BootDur, want)
	}
	if want := "Ready at: 2026-11-08 10:22:09"; v.BootDone != want {
		t.Errorf("ready = %q, want %q", v.BootDone, want)
	}
}

// TestViewHybridBoot is the reported field case: Fast Startup resumed the kernel
// session, so a kernel-uptime based number would be hours. The duration must be
// the log's few seconds, labelled "快速启动", and uptime restarts at logon.
func TestViewHybridBoot(t *testing.T) {
	ready := time.Date(2026, 11, 8, 12, 12, 23, 0, time.Local)
	now := ready.Add(5 * time.Minute)
	f := clock.NewFake(now, 113*time.Minute) // kernel counter still counts from the last cold boot

	v := app.New(f, locale.LangZh, report(now, ready, 3*time.Second, bootlog.KindHybrid)).View()
	if want := "本次开机耗时：3秒（快速启动）"; v.BootDur != want {
		t.Errorf("duration = %q, want %q", v.BootDur, want)
	}
	if want := "已运行：5分"; v.Up != want {
		t.Errorf("up = %q, want %q (must be since logon, not the kernel session)", v.Up, want)
	}
	if want := "开机完成：2026-11-08 12:12:23"; v.BootDone != want {
		t.Errorf("ready = %q, want %q", v.BootDone, want)
	}
}

// TestViewColdReference adds the "上次冷启动" line when the log supplied one.
func TestViewColdReference(t *testing.T) {
	ready := time.Date(2026, 11, 8, 12, 12, 23, 0, time.Local)
	now := ready.Add(time.Minute)
	f := clock.NewFake(now, time.Minute)

	rep := report(now, ready, 3*time.Second, bootlog.KindHybrid)
	rep.Cold = &bootlog.Cold{Duration: 13 * time.Second, At: ready.Add(-2 * time.Hour)}

	v := app.New(f, locale.LangZh, rep).View()
	if !strings.Contains(v.Hint, "上次冷启动 13秒") {
		t.Errorf("hint = %q, want the cold-boot reference", v.Hint)
	}
}

// TestViewFallback keeps the app honest when the event log is unreadable: no
// invented duration, and the hint says the number is a kernel-counter fallback.
func TestViewFallback(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 15, 0, 0, time.Local)
	f := clock.NewFake(now, 113*time.Minute)

	rep := bootlog.FromUptime(now, 113*time.Minute)
	v := app.New(f, locale.LangZh, rep).View()
	if want := "本次开机耗时：未知"; v.BootDur != want {
		t.Errorf("duration = %q, want %q", v.BootDur, want)
	}
	if want := "开机完成：未知"; v.BootDone != want {
		t.Errorf("ready = %q, want %q", v.BootDone, want)
	}
	if !strings.Contains(v.Hint, "偏大") {
		t.Errorf("hint = %q, want the fallback warning", v.Hint)
	}
}

// TestRunningAdvancesWithMonotonicClock: after the first reading the uptime must
// follow the monotonic counter, so an NTP step or a manual clock change cannot
// move it, while sleep time still counts.
func TestRunningAdvancesWithMonotonicClock(t *testing.T) {
	ready := time.Date(2026, 11, 8, 10, 0, 0, 0, time.Local)
	now := ready.Add(time.Minute)
	f := clock.NewFake(now, 30*time.Minute)

	a := app.New(f, locale.LangZh, report(now, ready, 14*time.Second, bootlog.KindCold))
	if got := a.View().Up; got != "已运行：1分" {
		t.Fatalf("initial up = %q", got)
	}

	f.Advance(4 * time.Hour) // sleep 4h: monotonic counter advances too
	if got := a.View().Up; got != "已运行：4小时1分" {
		t.Errorf("after sleep up = %q", got)
	}

	f.StepWall(-30 * time.Minute) // NTP correction: wall clock only
	if got := a.View().Up; got != "已运行：4小时1分" {
		t.Errorf("after wall step up = %q", got)
	}
}

// TestToastViewCarriesTheDuration: the popup's headline is the boot duration.
func TestToastViewCarriesTheDuration(t *testing.T) {
	ready := time.Date(2026, 11, 8, 12, 12, 23, 0, time.Local)
	now := ready.Add(2 * time.Second)
	f := clock.NewFake(now, 113*time.Minute)

	got := app.New(f, locale.LangZh, report(now, ready, 3*time.Second, bootlog.KindHybrid)).ToastView()
	if got.Header != "本次开机耗时（快速启动）" {
		t.Errorf("header = %q", got.Header)
	}
	if want := "3秒"; got.Duration != want {
		t.Errorf("duration = %q, want %q", got.Duration, want)
	}
	if want := "开机完成：2026-11-08 12:12:23"; got.Done != want {
		t.Errorf("done = %q, want %q", got.Done, want)
	}
	if got.Hint != "点击查看详情" {
		t.Errorf("hint = %q", got.Hint)
	}
}
