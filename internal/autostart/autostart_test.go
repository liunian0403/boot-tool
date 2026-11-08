//go:build windows

package autostart

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// scratchKey points runKeyPath at a throwaway key under HKCU\Software for the
// duration of a test, so the real per-user Run key is never touched.
func scratchKey(t *testing.T) {
	t.Helper()
	old := runKeyPath
	runKeyPath = fmt.Sprintf(`Software\BootTool-tests-%d`, os.Getpid())
	t.Cleanup(func() {
		_, _, _ = procRegDeleteKeyW.Call(
			hkeyCurrentUser,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(runKeyPath))),
		)
		runKeyPath = old
	})
}

func TestInstallReadUninstallRoundTrip(t *testing.T) {
	scratchKey(t)

	if cmd, ok, err := InstalledCommand(); err != nil || ok || cmd != "" {
		t.Fatalf("before install: cmd=%q ok=%v err=%v, want empty/false/nil", cmd, ok, err)
	}

	const exe = `C:\Users\someone\AppData\Local\Programs\BootTool\boot-tool.exe`
	if err := Install(exe); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	cmd, ok, err := InstalledCommand()
	if err != nil {
		t.Fatalf("InstalledCommand() error = %v", err)
	}
	if !ok {
		t.Fatal("InstalledCommand() installed = false, want true")
	}
	want := `"` + exe + `" --autostart`
	if cmd != want {
		t.Fatalf("command = %q, want %q", cmd, want)
	}
	if !strings.HasSuffix(cmd, AutostartFlag) {
		t.Errorf("command %q does not end with %q", cmd, AutostartFlag)
	}

	// Re-installing a different path must replace, not duplicate, and stay idempotent.
	if err := Install(`D:\Tools\boot-tool.exe`); err != nil {
		t.Fatalf("re-Install() error = %v", err)
	}
	if cmd, _, _ := InstalledCommand(); cmd != `"D:\Tools\boot-tool.exe" --autostart` {
		t.Fatalf("command after re-install = %q", cmd)
	}

	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if cmd, ok, err := InstalledCommand(); err != nil || ok || cmd != "" {
		t.Fatalf("after uninstall: cmd=%q ok=%v err=%v, want empty/false/nil", cmd, ok, err)
	}

	// Uninstalling twice is success: the requested end state already holds.
	if err := Uninstall(); err != nil {
		t.Fatalf("second Uninstall() error = %v", err)
	}
}

func TestInstallRejectsEmptyPath(t *testing.T) {
	if err := Install(""); err == nil {
		t.Fatal("Install(\"\") error = nil, want failure")
	}
}

func TestCommandQuotesPathWithSpaces(t *testing.T) {
	got := Command(`C:\Program Files\Boot Tool\boot-tool.exe`)
	want := `"C:\Program Files\Boot Tool\boot-tool.exe" --autostart`
	if got != want {
		t.Fatalf("Command() = %q, want %q", got, want)
	}
}
