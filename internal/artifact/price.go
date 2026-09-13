package artifact

// WithoutPrices prepares new output while leaving legacy input artifacts intact.
// The v1 price field remains readable for backward compatibility only.
func WithoutPrices(a Artifact) Artifact {
	a.Events = append([]Event{}, a.Events...)
	for i := range a.Events {
		a.Events[i].Price = nil
	}
	return a
}
