//go:build !windows

package autostart

import "errors"

// errUnsupported keeps the non-Windows developer builds honest: the delivery
// target is Windows, and silently pretending to register startup would hide
// that fact.
var errUnsupported = errors.New("autostart: only supported on Windows")

// Install is a no-op stub on non-Windows developer builds.
func Install(string) error { return errUnsupported }

// Uninstall is a no-op stub on non-Windows developer builds.
func Uninstall() error { return errUnsupported }

// InstalledCommand always reports "not registered" on non-Windows builds.
func InstalledCommand() (string, bool, error) { return "", false, nil }
