// Package app assembles the display strings for the window and the startup
// popup from a Clock and this session's boot record.
package app

import (
	"time"

	"github.com/liunian0403/boot-tool/internal/bootlog"
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/locale"
	"github.com/liunian0403/boot-tool/internal/tfmt"
)

// App produces display strings from a Clock and a boot record.
//
// The boot duration never comes from the clock: with Fast Startup the kernel
// counter keeps running across a power-off, so "uptime" can be hours while the
// machine booted seconds ago (docs/design.md §16).
type App struct {
	clk  clock.Clock
	lang locale.Lang
	boot bootlog.Report

	// anchorUp/anchorSince fix "how long since the desktop became ready" once,
	// then advance it with the monotonic counter, so wall-clock corrections
	// cannot move the number.
	anchorUp    time.Duration
	anchorSince time.Duration
}

// New wires a Clock, a UI language and this session's boot record.
func New(clk clock.Clock, lang locale.Lang, boot bootlog.Report) *App {
	a := &App{clk: clk, lang: lang, boot: boot}
	a.anchorUp = clk.Uptime()
	a.anchorSince = clk.Now().Sub(boot.Ready)
	if a.anchorSince < 0 {
		a.anchorSince = 0
	}
	return a
}

// View is everything the main window needs for one refresh.
type View struct {
	Title    string // window title
	Up       string // "已运行：3小时5分" — since the desktop became ready
	BootDur  string // "本次开机耗时：14秒"
	BootDone string // "开机完成：2026-11-08 10:22:09"
	Hint     string // caliber note, small print
}

// Toast is the one-shot bottom-right popup shown after an automatic start.
type Toast struct {
	Header   string // "本次开机耗时" / "本次开机耗时（快速启动）"
	Duration string // "14秒" — the headline value
	Done     string // "开机完成：2026-11-08 10:22:09"
	Hint     string // "点击查看详情"
}

// View reads the current state for the main window.
func (a *App) View() View {
	return View{
		Title:    a.title(),
		Up:       a.labelUp() + tfmt.Uptime(a.running(), a.lang),
		BootDur:  a.bootDurLine(),
		BootDone: a.bootDoneLine(),
		Hint:     a.hint(),
	}
}

// ToastView reads the current state for the startup popup.
func (a *App) ToastView() Toast {
	return Toast{
		Header:   a.toastHeader(),
		Duration: a.bootDurationValue(),
		Done:     a.bootDoneLine(),
		Hint:     a.toastHint(),
	}
}

// running is time since the desktop became ready for this boot session.
func (a *App) running() time.Duration {
	d := a.anchorSince + (a.clk.Uptime() - a.anchorUp)
	if d < 0 {
		return 0
	}
	return d
}

// bootDurationValue is the headline "本次开机耗时" value.
func (a *App) bootDurationValue() string {
	if a.boot.Approximate {
		return a.unknownValue()
	}
	return tfmt.Uptime(a.boot.Duration, a.lang)
}

func (a *App) bootDurLine() string {
	if a.lang == locale.LangZh {
		return "本次开机耗时：" + a.bootDurationValue() + a.kindSuffix()
	}
	return "Boot took: " + a.bootDurationValue() + a.kindSuffix()
}

func (a *App) bootDoneLine() string {
	if a.boot.Approximate {
		if a.lang == locale.LangZh {
			return "开机完成：未知"
		}
		return "Ready at: unknown"
	}
	if a.lang == locale.LangZh {
		return "开机完成：" + tfmt.Boot(a.boot.Ready.Local())
	}
	return "Ready at: " + tfmt.Boot(a.boot.Ready.Local())
}

// kindSuffix tells the user why a number may look small: a hybrid (Fast
// Startup) boot resumes a hibernated kernel session.
func (a *App) kindSuffix() string {
	switch a.boot.Kind {
	case bootlog.KindHybrid:
		if a.lang == locale.LangZh {
			return "（快速启动）"
		}
		return " (fast startup)"
	case bootlog.KindResume:
		if a.lang == locale.LangZh {
			return "（休眠恢复）"
		}
		return " (resumed from hibernation)"
	}
	return ""
}

func (a *App) unknownValue() string {
	if a.lang == locale.LangZh {
		return "未知"
	}
	return "unknown"
}

func (a *App) title() string {
	if a.lang == locale.LangZh {
		return "开机小助手"
	}
	return "Boot Assistant"
}

func (a *App) labelUp() string {
	if a.lang == locale.LangZh {
		return "已运行："
	}
	return "Uptime: "
}

func (a *App) hint() string {
	zh := "耗时取自系统启动日志（内核启动→登录完成），不含 BIOS 自检；已运行含睡眠/休眠"
	en := "Duration from the system boot log (kernel start→logon), excludes BIOS self-test; uptime includes sleep"
	if a.boot.Cold != nil && !a.boot.Approximate {
		if a.lang == locale.LangZh {
			zh = "上次冷启动 " + tfmt.Uptime(a.boot.Cold.Duration, a.lang) + "；" + zh
		} else {
			en = "Last cold boot " + tfmt.Uptime(a.boot.Cold.Duration, a.lang) + "; " + en
		}
	}
	if a.boot.Approximate {
		if a.lang == locale.LangZh {
			return "未读到系统启动日志，已运行按内核计时，快速启动后可能明显偏大"
		}
		return "System boot log unavailable: uptime falls back to the kernel counter, which Fast Startup can inflate"
	}
	if a.lang == locale.LangZh {
		return zh
	}
	return en
}

func (a *App) toastHeader() string {
	switch a.boot.Kind {
	case bootlog.KindHybrid:
		if a.lang == locale.LangZh {
			return "本次开机耗时（快速启动）"
		}
		return "Boot duration (fast startup)"
	case bootlog.KindResume:
		if a.lang == locale.LangZh {
			return "本次开机耗时（休眠恢复）"
		}
		return "Boot duration (resumed)"
	}
	if a.lang == locale.LangZh {
		return "本次开机耗时"
	}
	return "Boot duration"
}

func (a *App) toastHint() string {
	if a.lang == locale.LangZh {
		return "点击查看详情"
	}
	return "Click for details"
}
