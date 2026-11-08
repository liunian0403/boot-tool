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
