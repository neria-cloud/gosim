package contracts

import (
	"fmt"
	"math"
	"path"
	"strings"
)

func ValidatePorts(ports Ports) error {
	if ports.Clock == nil || ports.Events == nil || ports.Trace == nil || ports.RandomFor == nil {
		return fmt.Errorf("%w: incomplete provider ports", ErrInvalidConfig)
	}
	return nil
}

// OrderProviders validates an exact provider set and returns an owned slice in
// deterministic filesystem/network order. RT is supplied by the engine.
func OrderProviders(features FeatureSet, providers []Provider) ([]Provider, error) {
	if err := features.Validate(); err != nil {
		return nil, err
	}
	byFeature := make(map[Feature]Provider)
	for _, provider := range providers {
		if provider == nil {
			return nil, fmt.Errorf("%w: nil provider", ErrInvalidConfig)
		}
		feature := provider.Feature()
		if (feature != FS && feature != Net) || !features.Has(feature) || byFeature[feature] != nil {
			return nil, fmt.Errorf("%w: disabled, invalid or duplicate provider %d", ErrInvalidConfig, feature)
		}
		byFeature[feature] = provider
	}
	var ordered []Provider
	for _, feature := range []Feature{FS, Net} {
		if features.Has(feature) {
			if byFeature[feature] == nil {
				return nil, fmt.Errorf("%w: missing provider %d", ErrInvalidConfig, feature)
			}
			ordered = append(ordered, byFeature[feature])
		}
	}
	return ordered, nil
}

func ValidateBindings(owner Feature, bindings Bindings) error {
	if (owner == FS && bindings.Files != nil && bindings.Sockets == nil) ||
		(owner == Net && bindings.Sockets != nil && bindings.Files == nil) {
		return nil
	}
	return fmt.Errorf("%w: invalid per-epoch service ownership for feature %d", ErrInvalidConfig, owner)
}

func ValidateControls(owner Feature, controls ControlBindings) error {
	if (owner == FS && controls.FSControl != nil && controls.NetControl == nil) ||
		(owner == Net && controls.NetControl != nil && controls.FSControl == nil) {
		return nil
	}
	return fmt.Errorf("%w: invalid session control ownership for feature %d", ErrInvalidConfig, owner)
}

func MergeBindings(groups []Bindings) (Bindings, error) {
	var result Bindings
	for _, group := range groups {
		if group.Files != nil {
			if result.Files != nil {
				return Bindings{}, fmt.Errorf("%w: duplicate filesystem bindings", ErrInvalidConfig)
			}
			result.Files = group.Files
		}
		if group.Sockets != nil {
			if result.Sockets != nil {
				return Bindings{}, fmt.Errorf("%w: duplicate network bindings", ErrInvalidConfig)
			}
			result.Sockets = group.Sockets
		}
	}
	return result, nil
}

func ValidateFSConfig(config FSConfig) error {
	if !path.IsAbs(config.WorkingDir) || strings.ContainsRune(config.WorkingDir, 0) {
		return fmt.Errorf("%w: filesystem working directory must be an absolute path", ErrInvalidConfig)
	}
	if config.CapacityBytes%1024 != 0 || config.CapacityBytes > uint64(^uint(0)>>1) || config.LatencyNS < 0 {
		return fmt.Errorf("%w: invalid filesystem capacity or latency", ErrInvalidConfig)
	}
	return nil
}

func probability(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func ValidateFSFault(config FSFaultConfig) error {
	if !probability(config.Probability) || !probability(config.BeforeProbability) {
		return fmt.Errorf("%w: sync fault probabilities must be finite and in [0,1]", ErrInvalidConfig)
	}
	return nil
}

func validateLatency(minNS, maxNS int64, lambda float64) bool {
	return minNS >= 0 && maxNS >= minNS && !math.IsNaN(lambda) && !math.IsInf(lambda, 0) && lambda > 0
}

func ValidateNetConfig(config NetConfig) error {
	if config.TCPBufferSize < 0 || config.TCPBufferSize > MaxTCPBufferSize {
		return fmt.Errorf("%w: TCP buffer size must be between 0 and %d", ErrInvalidConfig, MaxTCPBufferSize)
	}

	if config.ConnectionTimeoutNS < 0 {
		return fmt.Errorf("%w: connection timeout must be non-negative", ErrInvalidConfig)
	}
	if !validateLatency(config.MinNS, config.MaxNS, config.Lambda) ||
		!probability(config.Fail) || !probability(config.Repair) {
		return fmt.Errorf("%w: invalid network latency/loss policy", ErrInvalidConfig)
	}
	return nil
}

func ValidateNetLink(link NetLink) error {
	if link.Direction > 1 {
		return fmt.Errorf("%w: direction must be Both or Forward", ErrInvalidConfig)
	}
	return nil
}

func ValidateNetLinkPolicy(policy NetLinkPolicy) error {
	if policy.SetLatency && !validateLatency(policy.MinNS, policy.MaxNS, policy.Lambda) ||
		policy.SetLoss && (!probability(policy.Fail) || !probability(policy.Repair)) {
		return fmt.Errorf("%w: invalid network link policy", ErrInvalidConfig)
	}
	return nil
}
