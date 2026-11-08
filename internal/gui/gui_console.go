//go:build !windows

// Package gui on non-Windows platforms is a development-only console mode
// (docs/design.md §3.2): it prints one snapshot per second so the core caliber
// logic can be smoke-tested on the dev machine.
package gui

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liunian0403/boot-tool/internal/app"
)

// Run prints a snapshot every second until interrupted. In popup mode it prints
// the popup content once and exits, mirroring the Windows autostart behaviour.
func Run(a *app.App, opt Options) error {
	if opt.Mode == ModeToast {
		t := a.ToastView()
		fmt.Printf("[toast] %s: %s | %s | %s\n", t.Header, t.Duration, t.Done, t.Hint)
		return nil
	}

	printView := func() {
		v := a.View()
		fmt.Printf("[%s] %s | %s | %s\n", v.Title, v.Up, v.BootDur, v.BootDone)
		fmt.Printf("        %s\n", v.Hint)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	printView()
	for {
		select {
		case <-ticker.C:
			printView()
		case <-sig:
			fmt.Println()
			return nil
		}
	}
}
