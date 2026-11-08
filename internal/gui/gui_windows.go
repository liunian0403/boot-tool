//go:build windows

// Package gui renders the app view in native Win32 windows: the main window for
// a manual launch and the bottom-right startup popup for an automatic start.
// Uses only syscall-level user32/gdi32 calls so the binary stays
// dependency-free and cross-compiles from any OS (docs/design.md §3.1).
package gui

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/diag"
)

// Mode, Options and the popup timing defaults live in options.go so the
// non-Windows developer console mode compiles too.

const (
	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsVisible     = 0x10000000
	wsChild       = 0x40000000
	wsPopup       = 0x80000000
	styleWindow   = wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox | wsVisible
	styleStatic   = wsChild | wsVisible

	// ssNotify makes a static control report clicks (STN_CLICKED) to its parent.
	ssNotify = 0x0100

	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExNoActivate  = 0x08000000
	csDropShadow    = 0x00020000
	colorBtnFace    = 15
	colorWindow     = 5
	colorWindowText = 8
	colorGrayText   = 17

	idcArrow = 32512
	idiApp   = 32512

	swShowNormal     = 1
	swShowNoActivate = 4

	cwUseDefault = 0x80000000

	fwNormal   = 400
	fwBold     = 700
	defaultSet = 1

	// errClassExists: RegisterClassExW rejects a duplicate class name, which is
	// benign for this single-window process.
	errClassExists = 1410

	// Window messages handled by the two window procedures.
	wmDestroy      = 0x0002
	wmSetFont      = 0x0030
	wmCommand      = 0x0111
	wmTimer        = 0x0113
	wmTimeChange   = 0x001A
	wmCtlColorStat = 0x0138
	wmAppShow      = 0x8001 // WM_APP + 1: another launch asked us to show ourselves
	wmAppToastShow = 0x8002 // WM_APP + 2: settle delay elapsed, show the popup
	wmAppToastHide = 0x8003 // WM_APP + 3: the popup has been up long enough

	// stnClicked is the static-control notification code carried in WM_COMMAND.
	stnClicked = 0

	// timerRefresh drives the main window's 1s live refresh. The popup does NOT
	// use a timer: see scheduleToastAuto for why (WM_TIMER starvation).
	timerRefresh = 1

	spiGetWorkArea = 0x0030
	smCXScreen     = 0
	smCYScreen     = 1

	swpNoSize     = 0x0001
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040
	hwndTopmost   = ^uintptr(0) // (HWND)-1

	windowW = 480
	windowH = 200

	toastW      = 360
	toastH      = 152
	toastMargin = 16
)

// Class names double as the FindWindow targets for single-instance activation
// (see instance_windows.go).
const (
	mainClassName  = "BootToolMainWindow"
	toastClassName = "BootToolToastWindow"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW     = user32.NewProc("RegisterClassExW")
	procCreateWindowExW      = user32.NewProc("CreateWindowExW")
	procDefWindowProcW       = user32.NewProc("DefWindowProcW")
	procDestroyWindow        = user32.NewProc("DestroyWindow")
	procPostQuitMessage      = user32.NewProc("PostQuitMessage")
	procGetMessageW          = user32.NewProc("GetMessageW")
	procTranslateMessage     = user32.NewProc("TranslateMessage")
	procDispatchMessageW     = user32.NewProc("DispatchMessageW")
	procSetTimer             = user32.NewProc("SetTimer")
	procKillTimer            = user32.NewProc("KillTimer")
	procSetWindowTextW       = user32.NewProc("SetWindowTextW")
	procShowWindow           = user32.NewProc("ShowWindow")
	procUpdateWindow         = user32.NewProc("UpdateWindow")
	procLoadCursorW          = user32.NewProc("LoadCursorW")
	procLoadIconW            = user32.NewProc("LoadIconW")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procSendMessageW         = user32.NewProc("SendMessageW")
	procSetProcessDPIAware   = user32.NewProc("SetProcessDPIAware")
	procGetSysColorBrush     = user32.NewProc("GetSysColorBrush")
	procGetSysColor          = user32.NewProc("GetSysColor")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	procSetWindowPos         = user32.NewProc("SetWindowPos")
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	procFindWindowW          = user32.NewProc("FindWindowW")
	procPostMessageW         = user32.NewProc("PostMessageW")
	procCreateMutexW         = kernel32.NewProc("CreateMutexW")

	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procSetBkColor       = gdi32.NewProc("SetBkColor")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ x, y int32 }

type rect struct{ left, top, right, bottom int32 }

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

var (
	view *app.App

	hInstance uintptr
	appIcon   uintptr
	appCursor uintptr

	hMain          uintptr // main window, created lazily
	hUp            uintptr // uptime label (large)
	hBoot          uintptr // boot-time label
	hHint          uintptr // small-print caliber note
	mainClassOn    bool
	mainClassTried bool

	hToast          uintptr // startup popup, created lazily
	hToastHeader    uintptr
	hToastDuration  uintptr
	hToastBoot      uintptr
	hToastHint      uintptr
	toastClassOn    bool
	toastClassTried bool

	toastTimeout time.Duration
	toastVisible bool

	fontLarge      uintptr
	fontNormal     uintptr
	fontSmall      uintptr
	fontToastLarge uintptr
)

// Run shows the window (ModeWindow) or the startup popup (ModeToast) and blocks
// until the process should exit. It returns the first stage failure — already
// logged and shown to the user — so main can exit non-zero instead of exiting
// silently (PRD F6/AC0).
func Run(a *app.App, options Options) error {
	view = a
	if options.ToastDelay <= 0 {
		options.ToastDelay = defaultToastDelay
	}
	if options.ToastTimeout <= 0 {
		options.ToastTimeout = defaultToastTimeout
	}
	toastTimeout = options.ToastTimeout

	procSetProcessDPIAware.Call() // crisp text on high-DPI screens

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hInstance = hInst
	appCursor, _, _ = procLoadCursorW.Call(0, idcArrow)
	appIcon, _, _ = procLoadIconW.Call(0, idiApp)

	// Fonts are cosmetic: a failure degrades to the system default face but must
	// not stop the app.
	fontLarge = createFont(-30, fwBold)
	fontNormal = createFont(-21, fwNormal)
	fontSmall = createFont(-17, fwNormal)
	fontToastLarge = createFont(-28, fwBold)
	if fontLarge == 0 || fontNormal == 0 || fontSmall == 0 || fontToastLarge == 0 {
		diag.Logf("fonts degraded to system default (large=%d normal=%d small=%d toast=%d)",
			fontLarge, fontNormal, fontSmall, fontToastLarge)
	}

	switch options.Mode {
	case ModeToast:
		diag.Logf("stage: toast-mode delay=%s timeout=%s", options.ToastDelay, options.ToastTimeout)
		if err := createToast(options); err != nil {
			return err
		}
	default:
		if err := showMainWindow(); err != nil {
			return err
		}
	}

	diag.Logf("stage: entering-message-loop")
	var m winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	diag.Logf("stage: message-loop-ended")
	return nil
}

// showMainWindow creates the main window on first use and shows it afterwards.
// It is also the handler for a second launch asking the running instance to come
// to the front.
func showMainWindow() error {
	if hMain != 0 {
		procShowWindow.Call(hMain, swShowNormal)
		procSetForegroundWindow.Call(hMain)
		return nil
	}
	if err := ensureMainClass(); err != nil {
		return err
	}

	v := view.View()
	title := utf16(v.Title)
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16(mainClassName))),
		uintptr(unsafe.Pointer(title)),
		styleWindow,
		cwUseDefault, 0,
		windowW, windowH,
		0, 0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		return diag.FailStage("window", fmt.Errorf("CreateWindowExW: %w", err))
	}
	hMain = hwnd
	diag.Logf("stage: window ok (hwnd=%d)", hwnd)

	hUp = createStatic(hwnd, v.Up, 16, 18, 440, 48, 0)
	hBoot = createStatic(hwnd, v.Boot, 16, 74, 440, 32, 0)
	hHint = createStatic(hwnd, v.Hint, 16, 112, 440, 44, 0)
	if hUp == 0 || hBoot == 0 || hHint == 0 {
		return diag.FailStage("controls", fmt.Errorf(
			"CreateWindowExW(STATIC): hwnd up=%d boot=%d hint=%d, %w",
			hUp, hBoot, hHint, syscall.GetLastError()))
	}
	diag.Logf("stage: controls ok")

	if fontLarge != 0 {
		sendWMSetFont(hUp, fontLarge)
	}
	if fontNormal != 0 {
		sendWMSetFont(hBoot, fontNormal)
	}
	if fontSmall != 0 {
		sendWMSetFont(hHint, fontSmall)
	}

	procShowWindow.Call(hwnd, swShowNormal)
	procSetForegroundWindow.Call(hwnd) // never open behind other windows
	procUpdateWindow.Call(hwnd)

	// Without the 1s timer the uptime would silently freeze at its first value —
	// wrong data is worse than a visible failure, so this stage is fatal like
	// every other one (docs/design.md §5.1).
	if r, _, err := procSetTimer.Call(hwnd, timerRefresh, 1000, 0); r == 0 {
		return diag.FailStage("timer", fmt.Errorf("SetTimer: %w", err))
	}
	diag.Logf("stage: timer ok")
	return nil
}

// ensureMainClass registers the main window class exactly once.
func ensureMainClass() error {
	if mainClassOn || mainClassTried {
		return nil
	}
	mainClassTried = true
	if err := registerClass(mainClassName, 0); err != nil {
		return diag.FailStage("window-class", err)
	}
	mainClassOn = true
	diag.Logf("stage: window-class ok")
	return nil
}

// registerClass registers one window class with the shared icons/cursor.
func registerClass(name string, style uint32) error {
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		style:         style,
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     hInstance,
		hIcon:         appIcon,
		hCursor:       appCursor,
		hbrBackground: colorBtnFace + 1,
		lpszClassName: utf16(name),
		hIconSm:       appIcon,
	}
	// Both windows share one window procedure that dispatches on class name.
	if name == toastClassName {
		wc.hbrBackground = colorWindow + 1
	}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		if errno, ok := err.(syscall.Errno); !ok || errno != errClassExists {
			return fmt.Errorf("RegisterClassExW(%s): %w", name, err)
		}
	}
	return nil
}

// wndProc is the single window procedure for both windows.
func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmAppShow: // a second launch asked the running instance to show itself
		if err := showMainWindow(); err != nil {
			diag.Logf("activation failed: %v", err)
		}
		if hToast != 0 {
			closeToast()
		}
		return 0
	case wmAppToastShow:
		if hwnd == hToast {
			showToast()
		}
		return 0
	case wmAppToastHide:
		if hwnd == hToast && toastVisible {
			diag.Logf("stage: toast-hidden (timeout)")
			closeToast()
		}
		return 0
	case wmCommand:
		if hwnd == hToast {
			if uint16((wparam>>16)&0xFFFF) == stnClicked {
				openMainFromToast()
			}
			return 0
		}
	case wmDestroy:
		if hwnd == hMain {
			procKillTimer.Call(hwnd, timerRefresh)
			hMain = 0
		}
		if hwnd == hToast {
			hToast = 0
			toastVisible = false
		}
		// The process lives as long as either window exists.
		if hMain == 0 && hToast == 0 {
			procPostQuitMessage.Call(0)
		}
		return 0
	case wmTimer, wmTimeChange: // 1s tick, or timezone/clock change
		if hwnd == hMain {
			refresh()
		}
		return 0
	case wmCtlColorStat:
		return staticColors(hwnd, wparam, lparam)
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
}

// staticColors paints static controls. The popup uses window colours (a light
// card), the main window the dialog face colour.
func staticColors(hwnd, hdc, control uintptr) uintptr {
	if hwnd == hToast {
		if control == hToastHeader || control == hToastHint {
			procSetTextColor.Call(hdc, sysColor(colorGrayText))
		} else {
			procSetTextColor.Call(hdc, sysColor(colorWindowText))
		}
		procSetBkColor.Call(hdc, sysColor(colorWindow))
		return sysBrush(colorWindow)
	}
	// Main window: gray small print, black otherwise.
	if control == hHint {
		procSetTextColor.Call(hdc, 0x808080)
	} else {
		procSetTextColor.Call(hdc, 0x000000)
	}
	procSetBkColor.Call(hdc, 0xF0F0F0) // matches COLOR_BTNFACE window background
	return sysBrush(colorBtnFace)
}

// refresh recomputes the view from the clock and repaints the labels.
func refresh() {
	if view == nil || hUp == 0 {
		return
	}
	v := view.View()
	setText(hUp, v.Up)
	setText(hBoot, v.Boot)
	setText(hHint, v.Hint)
}

func createStatic(parent uintptr, text string, x, y, w, h int32, style uintptr) uintptr {
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16("STATIC"))),
		uintptr(unsafe.Pointer(utf16(text))),
		styleStatic|style,
		uintptr(uint32(x)), uintptr(uint32(y)),
		uintptr(uint32(w)), uintptr(uint32(h)),
		parent,
		0,
		hInstance,
		0,
	)
	return hwnd
}

func createFont(height, weight int32) uintptr {
	face := utf16("Segoe UI")
	f, _, _ := procCreateFontW.Call(
		uintptr(uint32(height)), 0, 0, 0,
		uintptr(uint32(weight)),
		0, 0, 0,
		defaultSet, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(face)),
	)
	return f
}

func sendWMSetFont(hwnd, font uintptr) {
	procSendMessageW.Call(hwnd, wmSetFont, font, 1)
}

func setText(hwnd uintptr, s string) {
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16(s))))
}

func sysColor(index uintptr) uintptr {
	v, _, _ := procGetSysColor.Call(index)
	return v
}

func sysBrush(index uintptr) uintptr {
	b, _, _ := procGetSysColorBrush.Call(index)
	return b
}

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		// Invalid UTF-16 surrogates cannot occur in our own literals; fall
		// back to a safe empty string rather than crashing the UI.
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}
