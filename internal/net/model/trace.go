package model

import (
	"fmt"

	"github.com/neria-cloud/gosim/internal/contracts"
)

// Trace never inspects payloads or consumes randomness.
func (n *Network) traceNetwork(event string, attrs ...any) {
	fields := make([]contracts.Field, 0, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		key := fmt.Sprint(attrs[i])
		add := func(suffix string, value any) {
			fields = append(fields, contracts.Field{Key: key + suffix, Value: fmt.Sprint(value)})
		}
		switch value := attrs[i+1].(type) {
		case LatencyConfig:
			add(".min", value.Min)
			add(".max", value.Max)
			add(".lambda", value.Lambda)
		case LossConfig:
			add(".failProbability", value.FailProbability)
			add(".repairProbability", value.RepairProbability)
		default:
			add("", value)
		}
	}
	n.trace.Emit(contracts.EpochRef{}, "network."+event, fields)
}
