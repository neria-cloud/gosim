package model

import (
	"math"
	"reflect"
	"testing"
	"time"
)

type scriptedRandom struct {
	uniforms, exponentials []float64
	calls                  []string
}

func (r *scriptedRandom) Float64() float64 {
	r.calls = append(r.calls, "uniform")
	if len(r.uniforms) == 0 {
		panic("unexpected uniform draw")
	}
	v := r.uniforms[0]
	r.uniforms = r.uniforms[1:]
	return v
}

func (r *scriptedRandom) ExpFloat64() float64 {
	r.calls = append(r.calls, "exponential")
	if len(r.exponentials) == 0 {
		panic("unexpected exponential draw")
	}
	v := r.exponentials[0]
	r.exponentials = r.exponentials[1:]
	return v
}

func TestFaultDrawOrder(t *testing.T) {
	t.Parallel()
	rng := &scriptedRandom{uniforms: []float64{0.1, 0.1, 0.8, 0.9, 0.2, 0.9}}
	n := testNetwork(t, rng)
	key := edgeKey(testPair)
	s := n.getOrCreateLink(key)
	loss := LossConfig{FailProbability: 0.5, RepairProbability: 0.5}
	if n.advanceLink(key, s, loss) || s.Status != linkRandomPartition || s.Generation != 1 {
		t.Fatal("failure")
	}
	if n.advanceLink(key, s, loss) || s.Generation != 1 {
		t.Fatal("unsuccessful repair")
	}
	if !n.advanceLink(key, s, loss) || s.Status != linkHealthy || s.Generation != 1 {
		t.Fatal("repair")
	}
	if !n.advanceLink(key, s, loss) {
		t.Fatal("healthy send")
	}
	if len(rng.calls) != 6 {
		t.Fatal(rng.calls)
	}
	s.Explicit = true
	if n.advanceLink(key, s, loss) || len(rng.calls) != 6 {
		t.Fatal("explicit partition drew randomness")
	}
	r := &scriptedRandom{}
	if probability(r, 0) || !probability(r, 1) || len(r.calls) != 0 {
		t.Fatal("endpoint probabilities drew randomness")
	}
}

func TestLatencySampling(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		c      LatencyConfig
		sample float64
		want   time.Duration
	}{
		{"default", DefaultConfig().Latency, 1, 20 * time.Millisecond},
		{"cap", DefaultConfig().Latency, 9, 100 * time.Millisecond},
		{"quantize", LatencyConfig{Min: 1500 * time.Microsecond, Max: 6500 * time.Microsecond, Lambda: 2.5}, 1, 3500 * time.Microsecond},
		{"submillisecond", LatencyConfig{Min: 100, Max: 900, Lambda: 5}, 100, 100},
		{"zero", LatencyConfig{Lambda: 5}, 3, 0},
		{"fixed", LatencyConfig{Min: 123, Max: 123, Lambda: 5}, 1, 123},
		{"tiny_lambda", LatencyConfig{Max: math.MaxInt64, Lambda: math.SmallestNonzeroFloat64}, 1, math.MaxInt64},
		{"fractional_cap", LatencyConfig{Min: 123, Max: 1500123, Lambda: 1}, 1.9, 1000123},
		{"fractional_cap_hit", LatencyConfig{Min: 123, Max: 1500123, Lambda: 1}, 2, 1500123},
		{"max_duration", LatencyConfig{Max: math.MaxInt64, Lambda: 1}, 1, 9223372036854000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &scriptedRandom{exponentials: []float64{tc.sample}}
			if got := sampleLatency(r, tc.c); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			if !reflect.DeepEqual(r.calls, []string{"exponential"}) {
				t.Fatal(r.calls)
			}
		})
	}
	if got, saturated := saturatingArrival(math.MaxInt64-5, 10); got != math.MaxInt64 || !saturated {
		t.Fatal(got, saturated)
	}
	if got, saturated := saturatingArrival(10, 5); got != 15 || saturated {
		t.Fatal(got, saturated)
	}
}

func (*scriptedRandom) Uint64() uint64        { panic("unexpected integer draw") }
func (*scriptedRandom) Uint64N(uint64) uint64 { panic("unexpected bounded draw") }
