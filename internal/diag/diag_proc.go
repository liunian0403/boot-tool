//go:build !windows

package diag

import (
	"fmt"
	"os"
)

// Alert prints to stderr on non-Windows dev builds (there is no GUI there).
func Alert(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// Fatal reports an unrecovered panic on non-Windows dev builds.
func Fatal(r any) {
	Logf("FATAL panic: %v", r)
	fmt.Fprintf(os.Stderr, "panic: %v\n(log: %s)\n", r, Path())
}
