//go:build windows

package locale

import "syscall"

var procGetUserDefaultUILanguage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// detectSystemLang uses the Windows user UI language; primary language ID
// 0x04 (zh-CN/zh-TW/zh-HK...) selects Chinese, everything else English (AC7).
func detectSystemLang() Lang {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	langID := uint16(r)
	if langID&0x3ff == 0x04 {
		return LangZh
	}
	return LangEn
}
