//go:build !windows

// Package gui on non-Windows platforms is a development-only console mode
// (docs/design.md §3.2): it prints one snapshot per second so the core
// caliber logic can be smoke-tested on the dev machine.
package gui

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/diag"
)

// Run prints a snapshot every second until interrupted. Non-Windows builds
// have no window, so only the applicable stages are logged; it never fails
// and satisfies the same contract as the Windows implementation.
func Run(a *app.App) error {
	diag.Logf("non-Windows console mode: window stages not applicable")

	printView := func() {
		v := a.View()
		fmt.Printf("[%s] %s | %s\n", v.Title, v.Boot, v.Up)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	diag.Logf("stage: entering-message-loop")
	printView()
	for {
		select {
		case <-ticker.C:
			printView()
		case <-sig:
			fmt.Println()
			diag.Logf("stage: message-loop-ended")
			return nil
		}
	}
}
