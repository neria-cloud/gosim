package gosim

import (
	"fmt"
	"syscall"
	"time"

	"github.com/neria-cloud/gosim/internal/simulation"
	"github.com/neria-cloud/gosim/internal/simulation/network"
)

// LatencyConfig defines a capped exponential delay. Min and Max must be
// nonnegative with Min <= Max; Lambda must be finite and positive. The variable
// delay is quantized to milliseconds. Delivery remains ordered, so an earlier
// backlog can delay packets beyond a newly lowered Max.
type LatencyConfig struct {
	Min, Max time.Duration
	Lambda   float64
}

// LossConfig gives traffic-triggered probabilities in [0,1]. Failure partitions
// a host pair and discards pending packets in both directions. A later send can
// repair a random partition; explicit partitions require Repair. Idle time
// never changes link state. All packet kinds, including ACKs, participate.
type LossConfig struct {
	FailProbability, RepairProbability float64
}

// NetworkConfig holds the global network policy for a simulation.
// The zero value is invalid; use DefaultNetworkConfig as a starting point.
type NetworkConfig struct {
	Latency LatencyConfig
	Loss    LossConfig
}

// DefaultNetworkConfig returns the automatic defaults: 0–100ms latency with
// lambda 5, failure probability 0, and repair probability 1. Safe outside a simulation.
func DefaultNetworkConfig() NetworkConfig { return fromNetworkConfig(network.DefaultConfig()) }

func toNetworkConfig(c NetworkConfig) network.Config {
	return network.Config{
		Latency: network.LatencyConfig{Min: c.Latency.Min, Max: c.Latency.Max, Lambda: c.Latency.Lambda},
		Loss:    network.LossConfig{FailProbability: c.Loss.FailProbability, RepairProbability: c.Loss.RepairProbability},
	}
}

func fromNetworkConfig(c network.Config) NetworkConfig {
	return NetworkConfig{
		Latency: LatencyConfig{Min: c.Latency.Min, Max: c.Latency.Max, Lambda: c.Latency.Lambda},
		Loss:    LossConfig{FailProbability: c.Loss.FailProbability, RepairProbability: c.Loss.RepairProbability},
	}
}

func wrapNetworkError(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("gosim %s: %w", op, err)
}

func applyNetworkControl(op string, request network.ControlRequest) error {
	if !IsSim() {
		return wrapNetworkError(op, syscall.ENOSYS)
	}
	return wrapNetworkError(op, simulation.SyscallConfigureNetwork(&request))
}

// ConfigureNetwork atomically replaces the global policy for future sends.
// Existing local group snapshots, queued packets, and partition states remain.
// Invalid configuration returns EINVAL; outside simulation it returns ENOSYS.
func ConfigureNetwork(config NetworkConfig) error {
	return applyNetworkControl("ConfigureNetwork", network.ControlRequest{Kind: network.ReplaceGlobal, Config: toNetworkConfig(config)})
}

// SetMaxMessageLatency changes the global maximum latency for future sends.
func SetMaxMessageLatency(max time.Duration) error {
	return applyNetworkControl("SetMaxMessageLatency", network.ControlRequest{Kind: network.SetGlobalMaxLatency, Duration: max})
}

// SetMessageLatencyCurve changes the global exponential curve for future sends.
func SetMessageLatencyCurve(lambda float64) error {
	return applyNetworkControl("SetMessageLatencyCurve", network.ControlRequest{Kind: network.SetGlobalCurve, Lambda: lambda})
}

// SetFailRate changes the global failure probability without changing link state.
func SetFailRate(probability float64) error {
	return applyNetworkControl("SetFailRate", network.ControlRequest{Kind: network.SetGlobalFailRate, Probability: probability})
}

// SetRepairRate changes the global traffic-triggered repair probability.
func SetRepairRate(probability float64) error {
	return applyNetworkControl("SetRepairRate", network.ControlRequest{Kind: network.SetGlobalRepairRate, Probability: probability})
}

// SetLinkLatency sets a fixed delay (including zero) for both directions of an IPv4 pair.
// Its first local latency update snapshots the complete global latency group.
func SetLinkLatency(a, b string, delay time.Duration) error {
	return applyNetworkControl("SetLinkLatency", network.ControlRequest{Kind: network.SetPairLatency, A: a, B: b, Duration: delay})
}

// SetLinkMaxMessageLatency changes a pair's maximum, snapshotting the entire latency group
// on its first local update. Both endpoints must be IPv4 addresses.
func SetLinkMaxMessageLatency(a, b string, max time.Duration) error {
	return applyNetworkControl("SetLinkMaxMessageLatency", network.ControlRequest{Kind: network.SetPairMaxLatency, A: a, B: b, Duration: max})
}

// SetLinkMessageLatencyCurve changes a pair's curve, snapshotting the entire latency group
// on its first local update. Both endpoints must be IPv4 addresses.
func SetLinkMessageLatencyCurve(a, b string, lambda float64) error {
	return applyNetworkControl("SetLinkMessageLatencyCurve", network.ControlRequest{Kind: network.SetPairCurve, A: a, B: b, Lambda: lambda})
}

// SetLinkFailRate changes a pair's failure probability, snapshotting the entire loss group
// on its first local update. It does not repair the link. Endpoints must be IPv4 addresses.
func SetLinkFailRate(a, b string, probability float64) error {
	return applyNetworkControl("SetLinkFailRate", network.ControlRequest{Kind: network.SetPairFailRate, A: a, B: b, Probability: probability})
}

// SetLinkRepairRate changes a pair's repair probability, snapshotting the entire loss group
// on its first local update. Endpoints must be IPv4 addresses.
func SetLinkRepairRate(a, b string, probability float64) error {
	return applyNetworkControl("SetLinkRepairRate", network.ControlRequest{Kind: network.SetPairRepairRate, A: a, B: b, Probability: probability})
}

// ResetLinkLatency restores live global latency inheritance for an IPv4 pair.
func ResetLinkLatency(a, b string) error {
	return applyNetworkControl("ResetLinkLatency", network.ControlRequest{Kind: network.ResetPairLatency, A: a, B: b})
}

// ResetLinkLoss restores live global loss inheritance for an IPv4 pair without repairing it.
func ResetLinkLoss(a, b string) error {
	return applyNetworkControl("ResetLinkLoss", network.ControlRequest{Kind: network.ResetPairLoss, A: a, B: b})
}

// Partition explicitly partitions both directions of an IPv4 pair and discards
// pending packets. Automatic repair cannot override an explicit partition.
func Partition(a, b string) error {
	return applyNetworkControl("Partition", network.ControlRequest{Kind: network.SetConnectivity, A: a, B: b, Direction: network.DirectionTwoWay, Connected: false})
}

// Repair restores both directions of an IPv4 pair, retaining policy.
// It does not resurrect discarded packets or reopen connections.
func Repair(a, b string) error {
	return applyNetworkControl("Repair", network.ControlRequest{Kind: network.SetConnectivity, A: a, B: b, Direction: network.DirectionTwoWay, Connected: true})
}

// PartitionOneWay will partition only from→to. Currently valid IPv4 requests return
// EOPNOTSUPP without changing the network; outside simulation it returns ENOSYS.
// NotImplemented: operational one-way support is reserved for a future change.
func PartitionOneWay(from, to string) error {
	return applyNetworkControl("PartitionOneWay", network.ControlRequest{Kind: network.SetConnectivity, A: from, B: to, Direction: network.DirectionOneWay, Connected: false})
}

// RepairOneWay will repair only from→to, including after a two-way partition.
// Currently valid IPv4 requests return EOPNOTSUPP without changing the network;
// outside simulation it returns ENOSYS.
// NotImplemented: operational one-way support is reserved for a future change.
func RepairOneWay(from, to string) error {
	return applyNetworkControl("RepairOneWay", network.ControlRequest{Kind: network.SetConnectivity, A: from, B: to, Direction: network.DirectionOneWay, Connected: true})
}
