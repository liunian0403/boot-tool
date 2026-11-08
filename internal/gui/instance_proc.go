//go:build !windows

package gui

// AcquireSingleInstance always succeeds in the non-Windows developer console
// mode: there is no window to activate, so a second launch is harmless.
func AcquireSingleInstance() bool { return true }

// ActivateExisting has nothing to activate in the developer console mode.
func ActivateExisting() bool { return false }
