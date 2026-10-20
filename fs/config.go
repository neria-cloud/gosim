// Package fs controls the optional simulated filesystem provider.
package fs

import "time"

type Config struct {
	WorkingDir    string
	CapacityBytes uint64
	UID, GID      uint32
	Latency       time.Duration
}

func DefaultConfig() Config { return Config{WorkingDir: "/app", CapacityBytes: 128 << 20} }
