//go:build windows

// Package gui renders the app view in a native Win32 window. Uses only
// syscall-level user32/gdi32 calls so the binary stays dependency-free and
// cross-compiles from any OS (docs/design.md §3.1).
package gui

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/diag"
)

const (
	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsVisible     = 0x10000000
	wsChild       = 0x40000000
	styleWindow   = wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox | wsVisible
	styleStatic   = wsChild | wsVisible

	wmDestroy      = 0x0002
	wmSetFont      = 0x0030
	wmTimer        = 0x0113
	wmTimeChange   = 0x001A
	wmCtlColorStat = 0x0138

	colorBtnFace = 15
	idcArrow     = 32512
	idiApp       = 32512

	swShowNormal = 1
	fwNormal     = 400
	fwBold       = 700
	defaultSet   = 1

	// errClassExists: RegisterClassExW rejects a duplicate class name, which
	// is benign for this single-window process.
	errClassExists = 1410

	timerID = 1

	windowW = 480
	windowH = 200
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procSetTimer            = user32.NewProc("SetTimer")
	procKillTimer           = user32.NewProc("KillTimer")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	procGetSysColorBrush    = user32.NewProc("GetSysColorBrush")

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
	view      *app.App
	hUp       uintptr // uptime label (large)
	hBoot     uintptr // boot-time label
	hHint     uintptr // small-print caliber note
	hInstance uintptr
)

// Run shows the window and blocks until the user closes it. It follows the
// fixed startup stage sequence of docs/design.md §5.1
// (window-class → window → controls → timer → entering-message-loop) and
// returns the first stage failure — already logged and shown to the user —
// so main can exit non-zero instead of exiting silently (PRD F6/AC0).
func Run(a *app.App) error {
	view = a
	v := a.View()

	procSetProcessDPIAware.Call() // crisp text on high-DPI screens

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hInstance = hInst

	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	icon, _, _ := procLoadIconW.Call(0, idiApp)

	className := utf16("BootToolMainWindow")
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     hInst,
		hIcon:         icon,
		hCursor:       cursor,
		hbrBackground: colorBtnFace + 1,
		lpszClassName: className,
		hIconSm:       icon,
	}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		if errno, ok := err.(syscall.Errno); !ok || errno != errClassExists {
			return diag.FailStage("window-class", fmt.Errorf("RegisterClassExW: %w", err))
		}
	}
	diag.Logf("stage: window-class ok")

	title := utf16(v.Title)
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		styleWindow,
		0x80000000, // CW_USEDEFAULT for x/y
		0,
		windowW,
		windowH,
		0, 0,
		hInst,
		0,
	)
	if hwnd == 0 {
		return diag.FailStage("window", fmt.Errorf("CreateWindowExW: %w", err))
	}
	diag.Logf("stage: window ok (hwnd=%d)", hwnd)

	// Fonts: Segoe UI, the Windows 10/11 default UI face. A failed font is
	// cosmetic — degrade to the system default and keep running.
	fLarge := createFont(-30, fwBold)
	fNormal := createFont(-21, fwNormal)
	fSmall := createFont(-17, fwNormal)
	if fLarge == 0 || fNormal == 0 || fSmall == 0 {
		diag.Logf("fonts degraded to system default (large=%d normal=%d small=%d)", fLarge, fNormal, fSmall)
	}

	hUp = createStatic(hwnd, utf16(v.Up), 16, 18, 440, 48)
	hBoot = createStatic(hwnd, utf16(v.Boot), 16, 74, 440, 28)
	hHint = createStatic(hwnd, utf16(v.Hint), 16, 112, 440, 44)
	if hUp == 0 || hBoot == 0 || hHint == 0 {
		return diag.FailStage("controls", fmt.Errorf(
			"CreateWindowExW(STATIC): hwnd up=%d boot=%d hint=%d, %w",
			hUp, hBoot, hHint, syscall.GetLastError()))
	}
	diag.Logf("stage: controls ok")

	if fLarge != 0 {
		sendWMSetFont(hUp, fLarge)
	}
	if fNormal != 0 {
		sendWMSetFont(hBoot, fNormal)
	}
	if fSmall != 0 {
		sendWMSetFont(hHint, fSmall)
	}

	procShowWindow.Call(hwnd, swShowNormal)
	procSetForegroundWindow.Call(hwnd) // never open behind other windows
	procUpdateWindow.Call(hwnd)

	// Without the 1s timer the uptime would silently freeze at its first
	// value — wrong data is worse than a visible failure, so this stage is
	// fatal like every other one (docs/design.md §5.1).
	if r, _, err := procSetTimer.Call(hwnd, timerID, 1000, 0); r == 0 {
		return diag.FailStage("timer", fmt.Errorf("SetTimer: %w", err))
	}
	diag.Logf("stage: timer ok")

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

// wndProc is the single window procedure for the process.
func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmDestroy:
		procKillTimer.Call(hwnd, timerID)
		procPostQuitMessage.Call(0)
		return 0
	case wmTimer, wmTimeChange: // 1s tick, or timezone/clock change
		refresh()
		return 0
	case wmCtlColorStat:
		// Paint static controls: gray small print, black otherwise.
		hdc := wparam
		if lparam == hHint {
			procSetTextColor.Call(hdc, 0x808080)
		} else {
			procSetTextColor.Call(hdc, 0x000000)
		}
		procSetBkColor.Call(hdc, 0xF0F0F0) // matches COLOR_BTNFACE window background
		brush, _, _ := procGetSysColorBrush.Call(colorBtnFace)
		return brush
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
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

func createStatic(parent uintptr, text *uint16, x, y, w, h int32) uintptr {
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16("STATIC"))),
		uintptr(unsafe.Pointer(text)),
		styleStatic,
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

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		// Invalid UTF-16 surrogates cannot occur in our own literals; fall
		// back to a safe empty string rather than crashing the UI.
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}
