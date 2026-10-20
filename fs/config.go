// Package fs controls the optional simulated filesystem provider.
package fs

import "time"

const defaultCapacityMiB = 128

const bytesPerMiBShift = 20

type Config struct {
	WorkingDir    string
	CapacityBytes uint64
	UID, GID      uint32
	Latency       time.Duration
}

func DefaultConfig() Config {
	return Config{WorkingDir: "/app", CapacityBytes: defaultCapacityMiB << bytesPerMiBShift}
}
