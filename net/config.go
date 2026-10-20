// Package net controls the optional simulated network provider.
package net

import (
	"net/netip"
	"time"
)

const (
	defaultLatencyMilliseconds = 100
	defaultTCPBufferBytes      = 1024
	defaultLatencyLambda       = 5
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
		// ConnectionTimeout bounds connection establishment for new attempts.
		// Zero uses the default of one second; negative values are invalid.
		// Reconfiguration does not change pending attempts or established
		// connections. A shorter caller deadline still takes precedence.
		ConnectionTimeout time.Duration
		// TCPBufferSize is the per-direction byte capacity for new TCP connections.
		// Zero selects the default 1024 bytes. Values up to 16 MiB are supported.
		// Reconfiguration does not resize pending or established connections.
		TCPBufferSize int
	}
)
type MachineConfig struct{ Addr netip.Addr }

func DefaultConfig() Config {
	return Config{
		Latency: LatencyConfig{
			Max: defaultLatencyMilliseconds * time.Millisecond, Lambda: defaultLatencyLambda,
		},
		Loss:              LossConfig{RepairProbability: 1},
		ConnectionTimeout: time.Second,
		TCPBufferSize:     defaultTCPBufferBytes,
	}
}
