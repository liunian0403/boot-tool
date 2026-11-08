//go:build windows

package gui

import (
	"syscall"
	"time"
	"unsafe"
)

// instanceMutexName identifies the single running instance. It is session-local
// (no "Global\" prefix), which is what a per-user tray app wants: every
// interactive session may run its own copy.
const instanceMutexName = "BootToolSingleInstance"

// errorAlreadyExists is ERROR_ALREADY_EXISTS, returned by CreateMutexW when the
// named mutex is already open.
const errorAlreadyExists = 183

// AcquireSingleInstance reports whether this process is the first instance. The
// mutex handle is intentionally never closed: closing it would let a later
// launch believe it is first while this process is still running.
func AcquireSingleInstance() bool {
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16(instanceMutexName))))
	if h == 0 {
		// Cannot verify (e.g. a restrictive session policy): never block a
		// legitimate launch over it.
		return true
	}
	if e, ok := err.(syscall.Errno); ok && e == errorAlreadyExists {
		return false
	}
	return true
}

// ActivateExisting asks the running instance to bring its main window forward
// (the popup is replaced by the main window). Right after sign-in the first
// instance may not have created a window yet, so this retries briefly.
func ActivateExisting() bool {
	for attempt := 0; attempt < 10; attempt++ {
		for _, class := range []string{mainClassName, toastClassName} {
			hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(utf16(class))), 0)
			if hwnd == 0 {
				continue
			}
			if r, _, _ := procPostMessageW.Call(hwnd, wmAppShow, 0, 0); r != 0 {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
