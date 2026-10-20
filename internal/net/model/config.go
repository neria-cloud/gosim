package model

import (
	"math"
	"net/netip"
	"syscall"
	"time"
)

type LatencyConfig struct {
	Min, Max time.Duration
	Lambda   float64
}
type (
	LossConfig struct{ FailProbability, RepairProbability float64 }
	Config     struct {
		Latency           LatencyConfig
		Loss              LossConfig
		ConnectionTimeout time.Duration
	}
)

type ControlKind uint8

const (
	ReplaceGlobal ControlKind = iota + 1
	SetGlobalMaxLatency
	SetGlobalCurve
	SetGlobalFailRate
	SetGlobalRepairRate
	SetPairLatency
	SetPairMaxLatency
	SetPairCurve
	SetPairFailRate
	SetPairRepairRate
	ResetPairLatency
	ResetPairLoss
	SetConnectivity
)

type ConnectivityDirection uint8

const (
	DirectionTwoWay ConnectivityDirection = iota
	DirectionOneWay
)

// ControlRequest is copied across the syscall boundary. Only fields selected by
// Kind are read. A and B retain caller order (source and destination).
type ControlRequest struct {
	Kind        ControlKind
	A, B        string
	Config      Config
	Duration    time.Duration
	Lambda      float64
	Probability float64
	Direction   ConnectivityDirection
	Connected   bool
}

type (
	linkKey    struct{ A, B netip.Addr }
	linkStatus uint8
)

const (
	linkHealthy linkStatus = iota
	linkRandomPartition
	linkExplicitPartition
)

type linkState struct {
	Latency    *LatencyConfig
	Loss       *LossConfig
	Status     linkStatus
	Explicit   bool
	Generation uint64
}

func DefaultConfig() Config {
	return Config{Latency: LatencyConfig{Max: 100 * time.Millisecond, Lambda: 5}, Loss: LossConfig{RepairProbability: 1}, ConnectionTimeout: time.Second}
}

func validateProbability(p float64) error {
	if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
		return syscall.EINVAL
	}
	return nil
}

func validateLatency(c LatencyConfig) error {
	if c.Min < 0 || c.Max < c.Min || math.IsNaN(c.Lambda) || math.IsInf(c.Lambda, 0) || c.Lambda <= 0 {
		return syscall.EINVAL
	}
	return nil
}

func validateLoss(c LossConfig) error {
	if err := validateProbability(c.FailProbability); err != nil {
		return err
	}
	return validateProbability(c.RepairProbability)
}

func ValidateConfig(c Config) error {
	if c.ConnectionTimeout < 0 {
		return syscall.EINVAL
	}
	if err := validateLatency(c.Latency); err != nil {
		return err
	}
	return validateLoss(c.Loss)
}

func ValidateControl(r ControlRequest) error {
	switch r.Kind {
	case ReplaceGlobal:
		return ValidateConfig(r.Config)
	case SetGlobalMaxLatency, SetPairLatency, SetPairMaxLatency:
		if r.Duration < 0 {
			return syscall.EINVAL
		}
		return nil
	case SetGlobalCurve, SetPairCurve:
		return validateLatency(LatencyConfig{Lambda: r.Lambda})
	case SetGlobalFailRate, SetGlobalRepairRate, SetPairFailRate, SetPairRepairRate:
		return validateProbability(r.Probability)
	case ResetPairLatency, ResetPairLoss:
		return nil
	case SetConnectivity:
		if r.Direction != DirectionTwoWay && r.Direction != DirectionOneWay {
			return syscall.EINVAL
		}
		return nil
	default:
		return syscall.EINVAL
	}
}

func ParseHostPair(a, b string) (HostPair, error) {
	x, err := netip.ParseAddr(a)
	if err != nil || !x.Is4() {
		return HostPair{}, syscall.EINVAL
	}
	y, err := netip.ParseAddr(b)
	if err != nil || !y.Is4() {
		return HostPair{}, syscall.EINVAL
	}
	return HostPair{SourceHost: x, DestHost: y}, nil
}

// edgeKey deliberately retains caller order.
func edgeKey(p HostPair) linkKey { return linkKey{A: p.SourceHost, B: p.DestHost} }

func selectedEdges(p HostPair, direction ConnectivityDirection) ([]linkKey, error) {
	if !p.SourceHost.Is4() || !p.DestHost.Is4() || direction > DirectionOneWay {
		return nil, syscall.EINVAL
	}
	keys := []linkKey{edgeKey(p)}
	if direction == DirectionTwoWay && p.SourceHost != p.DestHost {
		keys = append(keys, edgeKey(p.Flip()))
	}
	return keys, nil
}

func (n *Network) effectiveConfig(s *linkState) Config {
	c := n.config
	if s != nil {
		if s.Latency != nil {
			c.Latency = *s.Latency
		}
		if s.Loss != nil {
			c.Loss = *s.Loss
		}
	}
	return c
}

func (n *Network) ApplyControl(r ControlRequest) error {
	if err := ValidateControl(r); err != nil {
		return err
	}
	local := r.Kind >= SetPairLatency && r.Kind <= ResetPairLoss
	var pair HostPair
	if local || r.Kind == SetConnectivity {
		var err error
		pair, err = ParseHostPair(r.A, r.B)
		if err != nil {
			return err
		}
	}
	if r.Kind == SetConnectivity {
		return n.SetConnectivity(pair, r.Connected, r.Direction)
	}
	if !local {
		c := n.config
		switch r.Kind {
		case ReplaceGlobal:
			c = r.Config
		case SetGlobalMaxLatency:
			c.Latency.Max = r.Duration
		case SetGlobalCurve:
			c.Latency.Lambda = r.Lambda
		case SetGlobalFailRate:
			c.Loss.FailProbability = r.Probability
		case SetGlobalRepairRate:
			c.Loss.RepairProbability = r.Probability
		}
		if err := ValidateConfig(c); err != nil {
			return err
		}
		if c.ConnectionTimeout == 0 {
			c.ConnectionTimeout = DefaultConfig().ConnectionTimeout
		}
		n.config = c
		n.traceNetwork("configure_global", "operation", r.Kind, "latency", c.Latency, "loss", c.Loss, "connectionTimeout", c.ConnectionTimeout, "scope", "global")
		return nil
	}
	keys, err := selectedEdges(pair, r.Direction)
	if err != nil {
		return err
	}
	staged := make([]linkState, len(keys))
	for i, key := range keys {
		if old := n.links[key]; old != nil {
			staged[i] = *old
		}
		if err := n.applyLocal(&staged[i], r); err != nil {
			return err
		}
	}
	for i, key := range keys {
		*n.getOrCreateLink(key) = staged[i]
	}
	for _, key := range keys {
		state := n.links[key]
		c := n.effectiveConfig(state)
		event := "configure_link"
		if r.Kind == ResetPairLatency || r.Kind == ResetPairLoss {
			event = "reset_group"
		}
		n.traceNetwork(event, "a", key.A, "b", key.B, "operation", r.Kind, "latency", c.Latency, "loss", c.Loss, "latencyLocal", state.Latency != nil, "lossLocal", state.Loss != nil)
	}
	return nil
}

func (n *Network) applyLocal(s *linkState, r ControlRequest) error {
	if r.Kind == ResetPairLatency || r.Kind == ResetPairLoss {
		if s != nil {
			if r.Kind == ResetPairLatency {
				s.Latency = nil
			} else {
				s.Loss = nil
			}
		}
		return nil
	}
	c := n.effectiveConfig(s)
	latencyChanged := false
	switch r.Kind {
	case SetPairLatency:
		c.Latency.Min, c.Latency.Max = r.Duration, r.Duration
		latencyChanged = true
	case SetPairMaxLatency:
		c.Latency.Max = r.Duration
		latencyChanged = true
	case SetPairCurve:
		c.Latency.Lambda = r.Lambda
		latencyChanged = true
	case SetPairFailRate:
		c.Loss.FailProbability = r.Probability
	case SetPairRepairRate:
		c.Loss.RepairProbability = r.Probability
	}
	if err := ValidateConfig(c); err != nil {
		return err
	}
	if latencyChanged {
		snapshot := c.Latency
		s.Latency = &snapshot
	} else {
		snapshot := c.Loss
		s.Loss = &snapshot
	}
	return nil
}

// SetLinkPolicy replaces complete copied groups atomically; nil leaves a group unchanged.
func (n *Network) SetLinkPolicy(p HostPair, direction ConnectivityDirection, latency *LatencyConfig, loss *LossConfig) error {
	keys, err := selectedEdges(p, direction)
	if err != nil {
		return err
	}
	if latency != nil {
		if err := validateLatency(*latency); err != nil {
			return err
		}
	}
	if loss != nil {
		if err := validateLoss(*loss); err != nil {
			return err
		}
	}
	for _, key := range keys {
		s := n.getOrCreateLink(key)
		if latency != nil {
			c := *latency
			s.Latency = &c
		}
		if loss != nil {
			c := *loss
			s.Loss = &c
		}
		config := n.effectiveConfig(s)
		n.traceNetwork("configure_link", "a", key.A, "b", key.B, "latency", config.Latency, "loss", config.Loss, "latencyLocal", s.Latency != nil, "lossLocal", s.Loss != nil)
	}
	return nil
}

func (n *Network) ResetLinkPolicy(p HostPair, direction ConnectivityDirection) error {
	keys, err := selectedEdges(p, direction)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if s := n.links[key]; s != nil {
			s.Latency = nil
			s.Loss = nil
			config := n.effectiveConfig(s)
			n.traceNetwork("configure_link", "a", key.A, "b", key.B, "latency", config.Latency, "loss", config.Loss, "latencyLocal", false, "lossLocal", false)
		}
	}
	return nil
}
