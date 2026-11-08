package clock

// New returns the platform clock. The error is non-nil only when the platform
// time source cannot be loaded at all (docs/design.md §5.2: QueryInterruptTime
// is available since Windows 8 and the product targets Windows 10 1809+, so
// this is a defensive path) — the caller must surface it rather than run with
// silently wrong data.
func New() (Clock, error) {
	return newClock()
}
