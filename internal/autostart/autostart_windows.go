//go:build windows

package autostart

import (
	"fmt"
	"syscall"
	"unsafe"
)

// runKeyPath is a variable so tests can point at a scratch key instead of the
// real per-user Run key (see autostart_test.go).
var runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

const (
	hkeyCurrentUser = 0x80000001
	keyQueryValue   = 0x0001
	keySetValue     = 0x0002
	regSZ           = 1
	errFileNotFound = 2
	// regCreateOptions: REG_OPTION_NON_VOLATILE, the regular persistent key.
	regCreateOptions = 0
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegDeleteKeyW    = advapi32.NewProc("RegDeleteKeyW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

// Install registers exePath to start at sign-in with the autostart flag. It is
// idempotent: the same value name is overwritten.
func Install(exePath string) error {
	if exePath == "" {
		return fmt.Errorf("autostart: empty executable path")
	}
	h, err := openKey(true, keySetValue)
	if err != nil {
		return err
	}
	defer closeKey(h)

	data := syscall.StringToUTF16(Command(exePath)) // NUL-terminated, as REG_SZ requires
	r, _, _ := procRegSetValueExW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(ValueName))),
		0,
		regSZ,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)*2),
	)
	if r != 0 {
		return fmt.Errorf("autostart: write HKCU\\%s\\%s: %w", runKeyPath, ValueName, syscall.Errno(r))
	}
	return nil
}

// Uninstall removes the registration. A missing value is success: the requested
// end state is "not registered".
func Uninstall() error {
	h, err := openKey(false, keySetValue)
	if err != nil {
		if isErr(err, errFileNotFound) {
			return nil
		}
		return err
	}
	defer closeKey(h)

	r, _, _ := procRegDeleteValueW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(ValueName))),
	)
	if r != 0 && syscall.Errno(r) != errFileNotFound {
		return fmt.Errorf("autostart: delete HKCU\\%s\\%s: %w", runKeyPath, ValueName, syscall.Errno(r))
	}
	return nil
}

// InstalledCommand returns the registered command line, if any. It never creates
// the key: a read has no business changing state.
func InstalledCommand() (command string, installed bool, err error) {
	h, err := openKey(false, keyQueryValue)
	if err != nil {
		if isErr(err, errFileNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	defer closeKey(h)

	name := syscall.StringToUTF16Ptr(ValueName)
	var typ, size uint32
	r, _, _ := procRegQueryValueExW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&typ)),
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 {
		if syscall.Errno(r) == errFileNotFound {
			return "", false, nil
		}
		return "", false, fmt.Errorf("autostart: read HKCU\\%s\\%s: %w", runKeyPath, ValueName, syscall.Errno(r))
	}
	if typ != regSZ || size == 0 {
		return "", false, nil
	}

	buf := make([]uint16, size/2+1) // +1 guards a missing terminator
	r, _, _ = procRegQueryValueExW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&typ)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 {
		return "", false, fmt.Errorf("autostart: read HKCU\\%s\\%s: %w", runKeyPath, ValueName, syscall.Errno(r))
	}
	return syscall.UTF16ToString(buf), true, nil
}

// openKey opens (and, when create is true, creates) the Run key with the given
// access mask.
func openKey(create bool, access uint32) (syscall.Handle, error) {
	var h syscall.Handle
	subkey := syscall.StringToUTF16Ptr(runKeyPath)
	if create {
		r, _, _ := procRegCreateKeyExW.Call(
			hkeyCurrentUser,
			uintptr(unsafe.Pointer(subkey)),
			0, 0, regCreateOptions,
			uintptr(access),
			0,
			uintptr(unsafe.Pointer(&h)),
			0,
		)
		if r != 0 {
			return 0, fmt.Errorf("autostart: open HKCU\\%s: %w", runKeyPath, syscall.Errno(r))
		}
		return h, nil
	}

	r, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(subkey)),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&h)),
	)
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return h, nil
}

func closeKey(h syscall.Handle) {
	_, _, _ = procRegCloseKey.Call(uintptr(h))
}

func isErr(err error, code syscall.Errno) bool {
	e, ok := err.(syscall.Errno)
	return ok && e == code
}
