package gosimruntime

// ProfileLabels returns the current simulated goroutine's diagnostic labels.
// Only their serialized text crosses the engine boundary.
//
//go:norace
func ProfileLabels() string { return getg().profileLabels }

// SetProfileLabels changes simulated labels without touching host profiling.
//
//go:norace
func SetProfileLabels(labels string) { getg().profileLabels = labels }
