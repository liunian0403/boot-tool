package clock

// New returns the platform clock.
func New() Clock {
	return newClock()
}
