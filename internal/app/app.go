// Package app assembles a display View from a Clock and a UI language.
package app

import (
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/locale"
	"github.com/liunian0403/boot-tool/internal/tfmt"
	"github.com/liunian0403/boot-tool/internal/uptime"
)

// App produces display strings from a Clock.
type App struct {
	clk  clock.Clock
	lang locale.Lang
}

// New wires a Clock and UI language.
func New(clk clock.Clock, lang locale.Lang) *App {
	return &App{clk: clk, lang: lang}
}

// View is everything the UI needs for one refresh.
type View struct {
	Title string // window title
	Up    string // "已开机：3天4小时5分6秒"
	Boot  string // "开机时间：2026-11-08 09:15:32"
	Hint  string // caliber note, small print
}

// View reads a fresh snapshot and formats it for the UI.
func (a *App) View() View {
	s := uptime.Read(a.clk)
	return View{
		Title: a.title(),
		Up:    a.labelUp() + tfmt.Uptime(s.Up, a.lang),
		Boot:  a.labelBoot() + tfmt.Boot(s.Boot),
		Hint:  a.hint(),
	}
}

func (a *App) title() string {
	if a.lang == locale.LangZh {
		return "开机小助手"
	}
	return "Boot Assistant"
}

func (a *App) labelUp() string {
	if a.lang == locale.LangZh {
		return "已开机："
	}
	return "Uptime: "
}

func (a *App) labelBoot() string {
	if a.lang == locale.LangZh {
		return "开机时间："
	}
	return "Boot time: "
}

func (a *App) hint() string {
	if a.lang == locale.LangZh {
		return "含睡眠/休眠时间，与 Windows 任务管理器口径一致"
	}
	return "Includes sleep/hibernate, same caliber as Windows Task Manager"
}
