// Package net controls the optional simulated network provider.
package net

import (
	"net/netip"
	"time"
)

type LatencyConfig struct {
	Min, Max time.Duration
	Lambda   float64
}
type (
	LossConfig struct{ FailProbability, RepairProbability float64 }
	Config     struct {
		Latency LatencyConfig
		Loss    LossConfig
	}
)
type MachineConfig struct{ Addr netip.Addr }

func DefaultConfig() Config {
	return Config{Latency: LatencyConfig{Max: 100 * time.Millisecond, Lambda: 5}, Loss: LossConfig{RepairProbability: 1}}
}
