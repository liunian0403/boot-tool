//go:build windows

// Package gui renders the app view in a native Win32 window. Uses only
// syscall-level user32/gdi32 calls so the binary stays dependency-free and
// cross-compiles from any OS (docs/design.md §3.1).
package gui

import (
	"syscall"
	"unsafe"

	"github.com/liunian0403/boot-tool/internal/app"
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

	swShow     = 5
	fwNormal   = 400
	fwBold     = 700
	defaultSet = 1

	timerID = 1

	windowW = 480
	windowH = 200
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procPostQuitMessage    = user32.NewProc("PostQuitMessage")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procSetTimer           = user32.NewProc("SetTimer")
	procKillTimer          = user32.NewProc("KillTimer")
	procSetWindowTextW     = user32.NewProc("SetWindowTextW")
	procShowWindow         = user32.NewProc("ShowWindow")
	procUpdateWindow       = user32.NewProc("UpdateWindow")
	procLoadCursorW        = user32.NewProc("LoadCursorW")
	procLoadIconW          = user32.NewProc("LoadIconW")
	procSendMessageW       = user32.NewProc("SendMessageW")
	procSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	procGetSysColorBrush   = user32.NewProc("GetSysColorBrush")

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

// Run shows the window and blocks until the user closes it.
func Run(a *app.App) {
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
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	title := utf16(v.Title)
	hwnd, _, _ := procCreateWindowExW.Call(
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
		return
	}

	// Fonts: Segoe UI, the Windows 10/11 default UI face.
	fLarge := createFont(-30, fwBold)
	fNormal := createFont(-21, fwNormal)
	fSmall := createFont(-17, fwNormal)

	hUp = createStatic(hwnd, utf16(v.Up), 16, 18, 440, 48)
	hBoot = createStatic(hwnd, utf16(v.Boot), 16, 74, 440, 28)
	hHint = createStatic(hwnd, utf16(v.Hint), 16, 112, 440, 44)

	sendWMSetFont(hUp, fLarge)
	sendWMSetFont(hBoot, fNormal)
	sendWMSetFont(hHint, fSmall)

	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	procSetTimer.Call(hwnd, timerID, 1000, 0) // 1s refresh

	// Message loop.
	var m winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
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
