//go:build !windows

package locale

import "os"

// detectSystemLang reads the standard locale environment variables
// (LC_ALL > LC_MESSAGES > LANG) — dev builds only.
func detectSystemLang() Lang {
	return langFromEnv(os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG"))
}
