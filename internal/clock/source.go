package clock

import (
	"fmt"
	"strings"
)

// candidate is one platform uptime source attempt: name is what the startup
// log reports when it wins, find reports whether the entry point resolves.
type candidate struct {
	name string
	find func() error
}

// choose returns the index of the first candidate whose entry point resolves.
//
// Resolution is ordered best-caliber-first (docs/design.md §2.1). It exists
// because a single hard-coded import is a field failure waiting to happen:
// kernel32.dll stopped exporting QueryInterruptTime on some Windows 11 builds
// (HYPE-46), which made the whole tool fail at the clock stage. Only when every
// candidate is missing does the clock stay a hard, visible failure.
func choose(cands []candidate) (int, error) {
	missing := make([]string, 0, len(cands))
	for i, c := range cands {
		if err := c.find(); err == nil {
			return i, nil
		} else {
			missing = append(missing, c.name+": "+err.Error())
		}
	}
	return -1, fmt.Errorf("no usable uptime source (%s)", strings.Join(missing, "; "))
}
