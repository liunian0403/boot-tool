package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/locale"
)

// TestViewZh exercises the full chain Clock → Snapshot → formatted View.
func TestViewZh(t *testing.T) {
	at := time.Date(2026, 11, 8, 17, 15, 32, 0, time.Local)
	f := clock.NewFake(at, 3*24*time.Hour+4*time.Hour+5*time.Minute+6*time.Second)

	v := app.New(f, locale.LangZh).View()
	if v.Title != "开机小助手" {
		t.Errorf("title = %q", v.Title)
	}
	if want := "已开机：3天4小时5分6秒"; v.Up != want {
		t.Errorf("up = %q, want %q", v.Up, want)
	}
	if want := "开机时间：2026-11-05 13:10:26"; v.Boot != want {
		t.Errorf("boot = %q, want %q", v.Boot, want)
	}
	if !strings.Contains(v.Hint, "睡眠") {
		t.Errorf("hint = %q", v.Hint)
	}
}

func TestViewEn(t *testing.T) {
	at := time.Date(2026, 11, 8, 17, 15, 32, 0, time.Local)
	f := clock.NewFake(at, time.Hour)

	v := app.New(f, locale.LangEn).View()
	if v.Title != "Boot Assistant" {
		t.Errorf("title = %q", v.Title)
	}
	if want := "Uptime: 1h"; v.Up != want {
		t.Errorf("up = %q, want %q", v.Up, want)
	}
	if want := "Boot time: 2026-11-08 16:15:32"; v.Boot != want {
		t.Errorf("boot = %q, want %q", v.Boot, want)
	}
}

// TestToastViewZh: the startup popup carries the duration as its headline and
// reuses the same caliber wording as the main window (PRD F8).
func TestToastViewZh(t *testing.T) {
	at := time.Date(2026, 11, 8, 11, 13, 50, 0, time.Local)
	f := clock.NewFake(at, 3*time.Hour+5*time.Minute+30*time.Second)

	got := app.New(f, locale.LangZh).ToastView()
	if got.Header != "本次开机用时" {
		t.Errorf("header = %q", got.Header)
	}
	if want := "3小时5分30秒"; got.Duration != want {
		t.Errorf("duration = %q, want %q", got.Duration, want)
	}
	if want := "开机时间：2026-11-08 08:08:20"; got.Boot != want {
		t.Errorf("boot = %q, want %q", got.Boot, want)
	}
	if got.Hint != "点击查看详情" {
		t.Errorf("hint = %q", got.Hint)
	}
}

// TestToastViewEn mirrors the popup assertions for English systems.
func TestToastViewEn(t *testing.T) {
	at := time.Date(2026, 11, 8, 11, 13, 50, 0, time.Local)
	f := clock.NewFake(at, 90*time.Minute)

	got := app.New(f, locale.LangEn).ToastView()
	if got.Header != "Boot duration" {
		t.Errorf("header = %q", got.Header)
	}
	if want := "1h 30m"; got.Duration != want {
		t.Errorf("duration = %q, want %q", got.Duration, want)
	}
	if want := "Boot time: 2026-11-08 09:43:50"; got.Boot != want {
		t.Errorf("boot = %q, want %q", got.Boot, want)
	}
	if got.Hint != "Click for details" {
		t.Errorf("hint = %q", got.Hint)
	}
}
