package model

import (
	"math"
	"net/netip"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
)

var testPair = HostPair{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")}

func testNetwork(t *testing.T, rng contracts.Random) *Network {
	t.Helper()
	events := newTestEvents()
	n := NewNetwork(events, events, rng, &testTrace{})
	t.Cleanup(func() { n.queue.drop(func(*Packet) bool { return true }) })
	return n
}

func control(t *testing.T, n *Network, r ControlRequest) {
	t.Helper()
	if err := n.ApplyControl(r); err != nil {
		t.Fatal(err)
	}
}

func localRequest(kind ControlKind) ControlRequest {
	return ControlRequest{Kind: kind, A: testPair.SourceHost.String(), B: testPair.DestHost.String()}
}

func TestConfigDefaultsValidation(t *testing.T) {
	t.Parallel()
	want := Config{Latency: LatencyConfig{Max: 100 * time.Millisecond, Lambda: 5}, Loss: LossConfig{RepairProbability: 1}, ConnectionTimeout: time.Second, TCPBufferSize: 1024}
	if DefaultConfig() != want {
		t.Fatal(DefaultConfig())
	}
	bad := []Config{{}}
	for _, v := range []float64{-1, 2, math.NaN(), math.Inf(1), math.Inf(-1)} {
		c := want
		c.Loss.FailProbability = v
		bad = append(bad, c)
		c = want
		c.Loss.RepairProbability = v
		bad = append(bad, c)
	}
	for _, v := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		c := want
		c.Latency.Lambda = v
		bad = append(bad, c)
	}
	c := want
	c.Latency.Min = -1
	bad = append(bad, c)
	c = want
	c.Latency.Min = c.Latency.Max + 1
	bad = append(bad, c)
	c = want
	c.ConnectionTimeout = -1
	bad = append(bad, c)
	for _, size := range []int{-1, contracts.MaxTCPBufferSize + 1} {
		c := want
		c.TCPBufferSize = size
		bad = append(bad, c)
	}
	for _, c := range bad {
		if ValidateConfig(c) != syscall.EINVAL {
			t.Fatalf("accepted %+v", c)
		}
	}
	c = want
	c.Latency.Min = math.MaxInt64
	c.Latency.Max = math.MaxInt64
	c.Latency.Lambda = math.SmallestNonzeroFloat64
	if err := ValidateConfig(c); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "host", "::1", "::ffff:10.0.0.1", "10.0.0.999"} {
		if _, err := ParseHostPair(bad, "10.0.0.2"); err != syscall.EINVAL {
			t.Fatal(bad, err)
		}
		if _, err := ParseHostPair("10.0.0.2", bad); err != syscall.EINVAL {
			t.Fatal(bad, err)
		}
	}
	pair, err := ParseHostPair("10.0.0.2", "10.0.0.1")
	if err != nil || pair != testPair.Flip() || edgeKey(pair) == edgeKey(testPair) {
		t.Fatal(pair, err)
	}
}

func TestConfigSnapshotsAndAtomicity(t *testing.T) {
	t.Parallel()
	n := testNetwork(t, &scriptedRandom{})
	key := edgeKey(testPair)
	if n.config != DefaultConfig() {
		t.Fatal("constructor defaults", n.config)
	}
	// Invalid cross-field updates must not allocate a local snapshot or record.
	c := DefaultConfig()
	c.Latency.Min = 20 * time.Millisecond
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: c})
	invalid := c
	invalid.ConnectionTimeout = -1
	if n.ApplyControl(ControlRequest{Kind: ReplaceGlobal, Config: invalid}) != syscall.EINVAL || n.config != c {
		t.Fatal("invalid timeout change was not atomic")
	}
	if n.ApplyControl(ControlRequest{Kind: SetGlobalMaxLatency, Duration: time.Millisecond}) != syscall.EINVAL || n.config != c {
		t.Fatal("invalid global change was not atomic")
	}
	r := localRequest(SetPairMaxLatency)
	r.Duration = time.Millisecond
	if n.ApplyControl(r) != syscall.EINVAL || len(n.links) != 0 {
		t.Fatal("invalid update allocated a record")
	}
	r = localRequest(SetPairCurve)
	r.Lambda = 2.5
	control(t, n, r)
	r = localRequest(SetPairFailRate)
	r.Probability = 0.125
	control(t, n, r)
	s := n.links[key]
	if s.Latency.Min != 20*time.Millisecond || s.Latency.Max != 100*time.Millisecond || s.Loss.RepairProbability != 1 {
		t.Fatal("incomplete group snapshot", s)
	}
	c.Latency = LatencyConfig{Max: time.Second, Lambda: 8}
	c.Loss = LossConfig{FailProbability: 0.9, RepairProbability: 0.4}
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: c})
	effective := n.effectiveConfig(s)
	if effective.Latency.Lambda != 2.5 || effective.Loss.FailProbability != 0.125 || effective.Loss.RepairProbability != 1 {
		t.Fatal(effective)
	}
	before := effective
	r = localRequest(SetPairMaxLatency)
	r.Duration = time.Millisecond
	if n.ApplyControl(r) != syscall.EINVAL || n.effectiveConfig(s) != before {
		t.Fatal("invalid update mutated snapshot")
	}
	s.Status = linkRandomPartition
	s.Generation = 7
	control(t, n, localRequest(ResetPairLatency))
	if s.Latency != nil || s.Loss == nil || n.effectiveConfig(s).Latency != c.Latency {
		t.Fatal("reset did not restore latency inheritance")
	}
	control(t, n, localRequest(ResetPairLoss))
	if s.Loss != nil || s.Status != linkRandomPartition || s.Generation != 7 || n.effectiveConfig(s) != c {
		t.Fatal("reset changed state")
	}
	if err := n.SetDelay(testPair, 0); err != nil {
		t.Fatal(err)
	}
	if s.Latency.Min != 0 || s.Latency.Max != 0 {
		t.Fatal("zero delay was not fixed")
	}
	// Replacing caller-owned values cannot change installed values.
	c.Latency.Lambda = 99
	if n.config.Latency.Lambda != 8 {
		t.Fatal("config alias")
	}
	for _, r := range []ControlRequest{{Kind: 255}, {Kind: SetGlobalCurve, Lambda: math.NaN()}, {Kind: SetGlobalFailRate, Probability: -1}, {Kind: SetPairLatency, A: "invalid", B: "10.0.0.1"}} {
		before := n.config
		state := *s
		if err := n.ApplyControl(r); err != syscall.EINVAL {
			t.Fatal(r, err)
		}
		if n.config != before || !reflect.DeepEqual(*s, state) {
			t.Fatal("invalid request mutated config")
		}
	}
}

func TestConfigIndependentGroupInheritance(t *testing.T) {
	t.Parallel()
	n := testNetwork(t, &scriptedRandom{})
	r := localRequest(SetPairCurve)
	r.Lambda = 3
	control(t, n, r)
	control(t, n, ControlRequest{Kind: SetGlobalRepairRate, Probability: 0.2})
	c := n.effectiveConfig(n.links[edgeKey(testPair)])
	if c.Loss.RepairProbability != 0.2 || c.Latency.Lambda != 3 {
		t.Fatal(c)
	}
	control(t, n, localRequest(ResetPairLatency))
	r = localRequest(SetPairRepairRate)
	r.Probability = 0.4
	control(t, n, r)
	control(t, n, ControlRequest{Kind: SetGlobalCurve, Lambda: 7})
	control(t, n, ControlRequest{Kind: SetGlobalFailRate, Probability: 0.5})
	c = n.effectiveConfig(n.links[edgeKey(testPair)])
	if c.Latency.Lambda != 7 || c.Loss.FailProbability != 0 || c.Loss.RepairProbability != 0.4 {
		t.Fatal(c)
	}
}
