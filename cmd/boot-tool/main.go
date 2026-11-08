package main

import (
	"os"
	"runtime"

	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/diag"
	"github.com/liunian0403/boot-tool/internal/gui"
	"github.com/liunian0403/boot-tool/internal/locale"
)

// version is the release version; CI overwrites it from the v* tag with
// -ldflags "-X main.version=..." so it never needs editing by hand
// (docs/design.md §6.2, ADR-3).
var version = "0.2.0"

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
//	version-banner → clock → locale → window-class → window → controls → timer → entering-message-loop
//
// A failed stage logs "stage: <name> FAILED: <err>", raises a dialog through
// diag.FailStage and returns exit code 1 — never a silent exit.
func run() int {
	diag.Logf("stage: version-banner boot-tool %s (%s/%s)", version, runtime.GOOS, runtime.GOARCH)

	clk, err := clock.New()
	if err != nil {
		diag.FailStage("clock", err)
		return 1
	}
	diag.Logf("stage: clock ok")

	lang := locale.Detect()
	diag.Logf("stage: locale %s", lang)

	if err := gui.Run(app.New(clk, lang)); err != nil {
		// gui.Run already reported the failing stage (log + dialog).
		diag.Logf("boot-tool %s exiting after failure", version)
		return 1
	}
	diag.Logf("boot-tool %s exiting normally", version)
	return 0
}
