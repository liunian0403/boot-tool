package main

import (
	"os"
	"runtime"
	"strings"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/autostart"
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/diag"
	"github.com/liunian0403/boot-tool/internal/gui"
	"github.com/liunian0403/boot-tool/internal/locale"
)

// version is the release version; CI overwrites it from the v* tag with
// -ldflags "-X main.version=..." so it never needs editing by hand
// (docs/design.md §6.2, ADR-3).
var version = "0.3.0"

// startupAction is a maintenance command requested on the command line.
type startupAction int

const (
	actionNone startupAction = iota
	actionInstallStartup
	actionUninstallStartup
	actionHelp
)

const usage = `开机小助手 / Boot Tool

用法 / Usage:
  boot-tool.exe                       打开主窗口 / open the main window
  boot-tool.exe --autostart           启动弹窗模式（登录自启用）
  boot-tool.exe --install-startup     设置开机自动启动 / enable start at sign-in
  boot-tool.exe --uninstall-startup   取消开机自动启动 / disable start at sign-in
  boot-tool.exe --help                显示本说明 / show this help

日志 / Log: %TEMP%\boot-tool.log`

// invocation is the parsed command line.
type invocation struct {
	mode    gui.Mode
	action  startupAction
	exePath string // this executable, used for the startup command
}

func main() {
	// Any panic must surface as a visible dialog, never a silent exit (AC0).
	defer func() {
		if r := recover(); r != nil {
			diag.Fatal(r)
			os.Exit(1)
		}
	}()
	os.Exit(run())
}

// run walks the fixed startup stage sequence of docs/design.md §5.1:
//
//	version-banner → single-instance → clock → locale → window-class → window
//	→ controls → timer → entering-message-loop
//
// Popup mode uses its own documented sequence (toast-mode → toast-class → toast
// → toast-controls → toast-delay → toast-shown → toast-hidden). A failed stage
// logs "stage: <name> FAILED: <err>", raises a dialog through diag.FailStage and
// returns exit code 1 — never a silent exit.
func run() int {
	diag.Logf("stage: version-banner boot-tool %s (%s/%s)", version, runtime.GOOS, runtime.GOARCH)

	inv := parseArgs(os.Args[1:])
	if inv.action != actionNone {
		return runAction(inv)
	}

	// A second launch must not create a second window: it hands over to the
	// running instance instead (a sign-in autostart copy is usually already up).
	if !gui.AcquireSingleInstance() {
		diag.Logf("stage: single-instance already running")
		if gui.ActivateExisting() {
			diag.Logf("stage: single-instance activated the running window")
		} else {
			diag.Logf("stage: single-instance running, no window to activate yet")
		}
		return 0
	}
	diag.Logf("stage: single-instance ok")

	clk, err := clock.New()
	if err != nil {
		diag.FailStage("clock", err)
		return 1
	}
	diag.Logf("stage: clock ok (source=%s)", clock.SourceName(clk))

	lang := locale.Detect()
	diag.Logf("stage: locale %s", lang)

	if err := gui.Run(app.New(clk, lang), gui.Options{Mode: inv.mode}); err != nil {
		// gui.Run already reported the failing stage (log + dialog).
		diag.Logf("boot-tool %s exiting after failure", version)
		return 1
	}
	diag.Logf("boot-tool %s exiting normally", version)
	return 0
}

// parseArgs reads the small command surface documented in `usage`. Unknown
// arguments are logged and ignored: a launcher script passing an extra token
// must never stop the app from starting.
func parseArgs(args []string) invocation {
	inv := invocation{mode: gui.ModeWindow}
	if exe, err := os.Executable(); err == nil {
		inv.exePath = exe
	} else {
		inv.exePath = os.Args[0]
	}

	for _, arg := range args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case autostart.AutostartFlag:
			inv.mode = gui.ModeToast
		case "--install-startup":
			inv.action = actionInstallStartup
		case "--uninstall-startup":
			inv.action = actionUninstallStartup
		case "--help", "-h", "/?":
			inv.action = actionHelp
		default:
			diag.Logf("ignoring unknown argument %q", arg)
		}
	}
	return inv
}

// runAction executes a maintenance command and reports its outcome in a dialog:
// these run without a window, so a dialog is the only feedback channel.
func runAction(inv invocation) int {
	switch inv.action {
	case actionInstallStartup:
		if err := autostart.Install(inv.exePath); err != nil {
			diag.FailStage("autostart-install", err)
			return 1
		}
		diag.Logf("stage: autostart-install ok: %s", autostart.Command(inv.exePath))
		diag.Info("已设置为开机自动启动：\r\n" + autostart.Command(inv.exePath) +
			"\r\n\r\n下次登录 Windows 后，右下角会自动显示本次开机用时。")
		return 0

	case actionUninstallStartup:
		if err := autostart.Uninstall(); err != nil {
			diag.FailStage("autostart-uninstall", err)
			return 1
		}
		diag.Logf("stage: autostart-uninstall ok")
		diag.Info("已取消开机自动启动。")
		return 0

	case actionHelp:
		diag.Info(usage)
		return 0
	}
	return 0
}
