// Package autostart owns the per-user "start at sign-in" registration that makes
// the app show its boot-duration popup after every boot (docs/PRD.md F7).
//
// It writes exactly one value under HKCU (no administrator rights, no machine
// wide state) and nothing else: HKCU\Software\Microsoft\Windows\CurrentVersion\Run
// is the Windows-standard per-user autostart location, visible and removable in
// Task Manager → Startup apps.
package autostart

// ValueName is the Run value name. Stable across versions so an upgrade replaces
// the same entry instead of piling up duplicates.
const ValueName = "BootTool"

// AutostartFlag is the flag the Run command passes. It lets the app distinguish
// an automatic sign-in start (popup only) from a manual double-click (main
// window), see docs/design.md §15.2.
const AutostartFlag = "--autostart"

// Command builds the Run value data for exePath: the quoted executable (the path
// contains spaces on a per-user install under %LOCALAPPDATA%) followed by the
// autostart flag.
func Command(exePath string) string {
	return `"` + exePath + `" ` + AutostartFlag
}
