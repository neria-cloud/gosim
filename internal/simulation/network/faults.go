package network

import (
	"math"
	"math/rand"
	"time"
)

type randomSource interface {
	Float64() float64
	ExpFloat64() float64
}

// Package-level math/rand is translated to Gosim's seeded randomness.
type runtimeRandom struct{}

func (runtimeRandom) Float64() float64    { return rand.Float64() }
func (runtimeRandom) ExpFloat64() float64 { return rand.ExpFloat64() }
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

func (n *Network) getOrCreateLinkLocked(key linkKey) *linkState {
	if s := n.links[key]; s != nil {
		return s
	}
	s := &linkState{}
	n.links[key] = s
	return s
}

func (n *Network) advanceLinkLocked(key linkKey, s *linkState, loss LossConfig) bool {
	if s.Status == linkExplicitPartition {
		return false
	}
	fail := probability(n.rng, loss.FailProbability)
	switch s.Status {
	case linkHealthy:
		if fail {
			n.partitionLocked(key, s, linkRandomPartition)
			return false
		}
		return true
	case linkRandomPartition:
		// The failure trial above is intentionally consumed even while partitioned.
		if !probability(n.rng, loss.RepairProbability) {
			return false
		}
		s.Status = linkHealthy
		traceNetwork("random_repair", "a", key.A, "b", key.B, "generation", s.Generation)
		return true
	default:
		panic("invalid network link state")
	}
}

func (n *Network) clearOrderingLocked(key linkKey) {
	if a := n.endpoints[key.A]; a != nil {
		delete(a.last, key.B)
	}
	if key.A != key.B {
		if b := n.endpoints[key.B]; b != nil {
			delete(b.last, key.A)
		}
	}
}

func (n *Network) partitionLocked(key linkKey, s *linkState, status linkStatus) int {
	if status != linkRandomPartition && status != linkExplicitPartition {
		panic("invalid partition status")
	}
	if s.Generation == math.MaxUint64 {
		panic("network generation exhausted")
	}
	s.Generation++
	s.Status = status
	dropped := n.queue.dropPair(key)
	n.clearOrderingLocked(key)
	event := "random_partition"
	if status == linkExplicitPartition {
		event = "manual_partition"
	}
	traceNetwork(event, "a", key.A, "b", key.B, "status", status, "generation", s.Generation, "dropped", dropped)
	return dropped
}
