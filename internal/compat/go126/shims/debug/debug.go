//go:build go1.26 && !go1.27

// Package debug rejects host garbage-collector controls in simulation.
package debug

import (
	"time"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func SetMemoryLimit(limit int64) int64 {
	gosimruntime.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "rt",
			Operation: "runtime/debug.SetMemoryLimit",
			Message:   "native GC memory limits are not modeled",
		},
	)
	panic("abort returned")
}

// BuildInfo describes application build metadata. Simulated machines have no
// application executable image, so ReadBuildInfo reports metadata unavailable.
type BuildInfo struct {
	GoVersion string
	Path      string
	Main      Module
	Deps      []*Module
	Settings  []BuildSetting
}
type Module struct {
	Path, Version, Sum string
	Replace            *Module
}
type BuildSetting struct{ Key, Value string }

func ReadBuildInfo() (*BuildInfo, bool) { return nil, false }

// GCStats permits dormant collectors to describe their sample storage.
type GCStats struct {
	LastGC         time.Time
	NumGC          int64
	PauseTotal     time.Duration
	Pause          []time.Duration
	PauseEnd       []time.Time
	PauseQuantiles []time.Duration
}

func ReadGCStats(stats *GCStats) {
	gosimruntime.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "rt",
			Operation: "runtime/debug.ReadGCStats",
			Message:   "host garbage collection metrics are not modeled",
		},
	)
}
