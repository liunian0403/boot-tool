// Package tfmt formats boot instants and uptime durations for display
// (docs/design.md §4).
package tfmt

import (
	"strconv"
	"strings"
	"time"

	"github.com/liunian0403/boot-tool/internal/locale"
)

// Boot formats an absolute instant in the receiver's location as
// "2006-01-02 15:04:05". Numeric and unambiguous in both languages.
func Boot(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// Uptime formats d as a human-readable duration.
//
// Chinese: "3天4小时5分6秒", English: "3d 4h 5m 6s".
// Rules (docs/design.md §4): units run days→seconds with second precision
// always shown; trailing zero units are dropped; interior zeros are kept
// ("3天0小时4分11s"); an all-zero value renders "0秒"/"0s".
func Uptime(d time.Duration, lang locale.Lang) string {
	if d < 0 {
		d = 0
	}
	total := int64(d / time.Second)
	units := [4][2]string{
		{"天", "d"},  // days
		{"小时", "h"}, // hours
		{"分", "m"},  // minutes
		{"秒", "s"},  // seconds
	}
	vals := [4]int64{
		total / 86400,
		total % 86400 / 3600,
		total % 3600 / 60,
		total % 60,
	}
	end := 4
	for end > 1 && vals[end-1] == 0 {
		end--
	}
	start := 0
	for start < end && vals[start] == 0 {
		start++ // drop leading zero units; keep interior ones
	}
	if start >= end { // all zero
		start, end = 3, 4 // render "0秒"/"0s"
	}
	parts := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		unit := units[i][1] // en tag
		if lang == locale.LangZh {
			unit = units[i][0]
		}
		parts = append(parts, strconv.FormatInt(vals[i], 10)+unit)
	}
	sep := " "
	if lang == locale.LangZh {
		sep = ""
	}
	return strings.Join(parts, sep)
}
