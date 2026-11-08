package main

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/autostart"
	"github.com/liunian0403/boot-tool/internal/bootlog"
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/diag"
	"github.com/liunian0403/boot-tool/internal/gui"
	"github.com/liunian0403/boot-tool/internal/locale"
)

// version is the release version; CI overwrites it from the v* tag with
// -ldflags "-X main.version=..." so it never needs editing by hand
// (docs/design.md §6.2, ADR-3).
var version = "0.4.1"

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
			return 0
		}
		// Silent exit here is exactly what makes a double-click look like a
		// hang: the instance exists but no window could be reached (still
		// starting, or stuck). Say so instead.
		diag.Logf("stage: single-instance running but no window to activate")
		diag.Info("已有一个「开机小助手」正在运行，但没能找到它的窗口。\r\n" +
			"它可能正在启动；若一直如此，可在任务管理器中结束 boot-tool.exe 后重试。\r\n\r\n" +
			"An instance is already running but its window could not be reached.")
		return 0
	}
	diag.Logf("stage: single-instance ok")

	clk, err := clock.New()
	if err != nil {
		diag.FailStage("clock", err)
		return 1
	}
	diag.Logf("stage: clock ok (source=%s)", clock.SourceName(clk))

	// 本次开机耗时来自系统启动日志；读不到才回退为内核计时并在界面标注
	// （快速启动会让内核计数跨关机连续累计，docs/design.md §16）。
	boot, bootErr := readBootRecord(bootRecordBudget)
	if bootErr != nil {
		diag.Logf("stage: boot-record FAILED: %v (falling back to the kernel counter)", bootErr)
		boot = bootlog.FromUptime(clk.Now(), clk.Uptime())
	} else {
		diag.Logf("stage: boot-record ok (kind=%s start=%s ready=%s duration=%s source=%s)",
			boot.Kind,
			boot.Start.Format(time.RFC3339),
			boot.Ready.Format(time.RFC3339),
			boot.Duration.Round(time.Millisecond),
			boot.Source)
		if boot.Cold != nil {
			diag.Logf("stage: boot-record cold-reference duration=%s at=%s",
				boot.Cold.Duration.Round(time.Millisecond), boot.Cold.At.Format(time.RFC3339))
		}
	}

	lang := locale.Detect()
	diag.Logf("stage: locale %s", lang)

	if err := gui.Run(app.New(clk, lang, boot), gui.Options{Mode: inv.mode}); err != nil {
		// gui.Run already reported the failing stage (log + dialog).
		diag.Logf("boot-tool %s exiting after failure", version)
		return 1
	}
	diag.Logf("boot-tool %s exiting normally", version)
	return 0
}

// bootRecordBudget bounds the startup cost of reading the System event log. The
// query normally takes tens of milliseconds, but right after sign-in the log
// file may be cold and an antivirus scan can hold it, and the window must not be
// delayed by it — a multi-second gap before any window appears is indistinguishable
// from a hang. Past the budget the session falls back to the kernel counter and
// says so in the UI.
const bootRecordBudget = 1200 * time.Millisecond

// readBootRecord runs the event-log read with a deadline.
func readBootRecord(budget time.Duration) (bootlog.Report, error) {
	type result struct {
		rep bootlog.Report
		err error
	}
	ch := make(chan result, 1)
	go func() {
		rep, err := bootlog.Read()
		ch <- result{rep: rep, err: err}
	}()

	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.rep, r.err
	case <-timer.C:
		return bootlog.Report{}, errBootRecordSlow
	}
}

// errBootRecordSlow is reported when the System log could not be read in time.
var errBootRecordSlow = errors.New("boot record read exceeded " + bootRecordBudget.String())

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
