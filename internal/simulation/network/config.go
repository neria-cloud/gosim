package network

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
		Latency LatencyConfig
		Loss    LossConfig
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
	Generation uint64
}

func DefaultConfig() Config {
	return Config{Latency: LatencyConfig{Max: 100 * time.Millisecond, Lambda: 5}, Loss: LossConfig{RepairProbability: 1}}
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

func canonicalPair(p HostPair) linkKey {
	a, b := p.SourceHost, p.DestHost
	if b.Less(a) {
		a, b = b, a
	}
	return linkKey{A: a, B: b}
}

func (n *Network) effectiveConfigLocked(s *linkState) Config {
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
	n.mu.Lock()
	defer n.mu.Unlock()
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
		n.config = c
		traceNetwork("configure_global", "operation", r.Kind, "latency", c.Latency, "loss", c.Loss, "scope", "global")
		return nil
	}
	key := canonicalPair(pair)
	s := n.links[key]
	if r.Kind == ResetPairLatency || r.Kind == ResetPairLoss {
		if s != nil {
			if r.Kind == ResetPairLatency {
				s.Latency = nil
			} else {
				s.Loss = nil
			}
		}
		c := n.effectiveConfigLocked(s)
		traceNetwork("reset_group", "a", key.A, "b", key.B, "operation", r.Kind, "latency", c.Latency, "loss", c.Loss, "latencyLocal", s != nil && s.Latency != nil, "lossLocal", s != nil && s.Loss != nil)
		return nil
	}
	c := n.effectiveConfigLocked(s)
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
	s = n.getOrCreateLinkLocked(key)
	if latencyChanged {
		snapshot := c.Latency
		s.Latency = &snapshot
	} else {
		snapshot := c.Loss
		s.Loss = &snapshot
	}
	traceNetwork("configure_link", "a", key.A, "b", key.B, "operation", r.Kind, "latency", c.Latency, "loss", c.Loss, "latencyLocal", s.Latency != nil, "lossLocal", s.Loss != nil)
	return nil
}
