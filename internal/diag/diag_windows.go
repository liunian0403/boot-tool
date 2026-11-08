//go:build windows

package diag

import (
	"fmt"
	"runtime/debug"
	"syscall"
	"unsafe"
)

var user32MessageBox = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")

// Alert shows a modal error dialog: the GUI subsystem has no console, so this
// is the only way to reach the user. MB_SYSTEMMODAL keeps it on top of the
// (possibly never created) main window.
func Alert(msg string) {
	const ( // MB_OK | MB_ICONERROR | MB_SYSTEMMODAL
		mbOk          = 0x0
		mbIconError   = 0x10
		mbSystemModal = 0x1000
	)
	title := utf16("开机小助手 / Boot Tool")
	body := utf16(msg)
	_, _, _ = user32MessageBox.Call(
		0,
		uintptr(unsafe.Pointer(body)),
		uintptr(unsafe.Pointer(title)),
		mbOk|mbIconError|mbSystemModal,
	)
}

// Fatal reports an unrecovered panic: stack to the log first, then a dialog
// the user can act on. Called from main's recover; returning lets main exit
// non-zero.
func Fatal(r any) {
	Logf("FATAL panic: %v\n%s", r, debug.Stack())
	Alert(fmt.Sprintf(
		"程序发生内部错误 / Internal error\r\n\r\n%v\r\n\r\n日志 / Log: %s\r\n\r\n请将日志反馈给开发者 / Please send this log to the developer.",
		r, Path()))
}

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		// Invalid UTF-16 sequences cannot occur in our own literals; fall
		// back to a safe empty string rather than failing the dialog.
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}
