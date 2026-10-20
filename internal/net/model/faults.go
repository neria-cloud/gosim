package model

import (
	"math"
	"time"
)

type randomSource interface {
	Float64() float64
	ExpFloat64() float64
}

func probability(r randomSource, p float64) bool {
	if p == 0 {
		return false
	}
	if p == 1 {
		return true
	}
	return r.Float64() < p
}

func sampleLatency(r randomSource, c LatencyConfig) time.Duration {
	sample := r.ExpFloat64() // Also sampled for a fixed delay, defining draw order.
	rangeNS := int64(c.Max - c.Min)
	rangeMS := rangeNS / int64(time.Millisecond)
	if rangeMS == 0 {
		return c.Min
	}
	scaledMS := float64(rangeMS) * (sample / c.Lambda)
	capMS := (rangeNS-1)/int64(time.Millisecond) + 1
	if math.IsInf(scaledMS, 1) || scaledMS >= float64(capMS) {
		return c.Max
	}
	delta := time.Duration(int64(math.Floor(scaledMS))) * time.Millisecond
	if delta > c.Max-c.Min {
		return c.Max
	}
	return c.Min + delta
}

func saturatingArrival(now int64, delay time.Duration) (int64, bool) {
	if delay > 0 && now > math.MaxInt64-int64(delay) {
		return math.MaxInt64, true
	}
	return now + int64(delay), false
}

func (n *Network) getOrCreateLink(key linkKey) *linkState {
	if s := n.links[key]; s != nil {
		return s
	}
	s := &linkState{}
	n.links[key] = s
	return s
}

func (n *Network) advanceLink(key linkKey, s *linkState, loss LossConfig) bool {
	if s.Explicit {
		return false
	}
	fail := probability(n.rng, loss.FailProbability)
	switch s.Status {
	case linkHealthy:
		if fail {
			n.partition(key, s, linkRandomPartition)
			return false
		}
		return true
	case linkRandomPartition:
		// The failure trial above is intentionally consumed even while partitioned.
		if !probability(n.rng, loss.RepairProbability) {
			return false
		}
		s.Status = linkHealthy
		n.traceNetwork("random_repair", "a", key.A, "b", key.B, "generation", s.Generation)
		return true
	default:
		panic("invalid network link state")
	}
}

func (n *Network) clearOrdering(key linkKey) {
	if a := n.endpoints[key.A]; a != nil {
		delete(a.last, key.B)
	}
}

func (n *Network) partition(key linkKey, s *linkState, status linkStatus) int {
	if status != linkRandomPartition && status != linkExplicitPartition {
		panic("invalid partition status")
	}
	if s.Generation == math.MaxUint64 {
		panic("network generation exhausted")
	}
	s.Generation++
	if status == linkExplicitPartition {
		s.Explicit = true
	} else {
		s.Status = status
	}
	dropped := n.queue.dropPair(key)
	n.clearOrdering(key)
	event := "random_partition"
	if status == linkExplicitPartition {
		event = "manual_partition"
	}
	n.traceNetwork(event, "a", key.A, "b", key.B, "status", status, "generation", s.Generation, "dropped", dropped)
	return dropped
}
