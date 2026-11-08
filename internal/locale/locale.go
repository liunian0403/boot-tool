package locale

import "strings"

// Lang is the UI language.
type Lang int

// Supported UI languages.
const (
	LangEn Lang = iota
	LangZh
)

// String returns the BCP47-ish tag for the language.
func (l Lang) String() string {
	if l == LangZh {
		return "zh"
	}
	return "en"
}

// Detect returns the UI language for this machine.
func Detect() Lang {
	return detectSystemLang()
}

// langFromEnv inspects locale environment variables (non-Windows helper).
func langFromEnv(vals ...string) Lang {
	for _, v := range vals {
		if v == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(v), "zh") {
			return LangZh
		}
		return LangEn
	}
	return LangEn
}
