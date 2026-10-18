package network

import (
	"math"
	"net/netip"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func attachTestEndpoints(n *Network, hosts ...netip.Addr) {
	for _, addr := range hosts {
		n.endpoints[addr] = &Endpoint{net: n, addr: addr, stack: &Stack{}, last: make(map[netip.Addr]timeIndex)}
	}
}

func sendTestPacket(n *Network, pair HostPair) *Packet {
	p := allocPacket(PacketKindStreamData, ConnId{Hosts: pair})
	n.endpoints[pair.SourceHost].Send(p)
	return p
}

func TestNetworkPartitionAdmissionAndOrdering(t *testing.T) {
	rng := &scriptedRandom{exponentials: []float64{1, 0, 1, 0}}
	n := testNetwork(t, rng)
	attachTestEndpoints(n, testPair.SourceHost, testPair.DestHost)
	if err := n.SetDelay(testPair, time.Hour); err != nil {
		t.Fatal(err)
	}
	first := sendTestPacket(n, testPair)
	// Lowered latency applies immediately, but a prior arrival remains a floor.
	if err := n.SetDelay(testPair, 0); err != nil {
		t.Fatal(err)
	}
	second := sendTestPacket(n, testPair)
	if first.ArrivalTime.time != second.ArrivalTime.time || first.ArrivalTime.index >= second.ArrivalTime.index {
		t.Fatal("delivery reordered")
	}
	// Model a packet already dequeued but not yet admitted. No queue/network
	// goroutines are running in this owner-level fixture.
	n.queue.mu.Lock()
	n.queue.items = nil
	n.queue.updateTimerLocked()
	n.queue.mu.Unlock()
	admitted := n.getStack(first)
	if admitted == nil {
		t.Fatal("healthy packet not admitted")
	}
	if err := n.SetConnected(testPair, false); err != nil {
		t.Fatal(err)
	}
	if err := n.SetConnected(testPair.Flip(), true); err != nil {
		t.Fatal(err)
	}
	if n.getStack(second) != nil {
		t.Fatal("stale packet admitted after repair")
	}
	// Admission already granted before partition is irrevocable; the runner
	// uses this stack reference without a second state check or network lock.
	if admitted != n.endpoints[testPair.DestHost].stack {
		t.Fatal("admission lost")
	}
	freePacket(first)
	freePacket(second)
	if len(n.endpoints[testPair.SourceHost].last) != 0 {
		t.Fatal("watermark survived partition")
	}
	third := sendTestPacket(n, testPair)
	if third.Generation != 1 || third.ArrivalTime.time > time.Now().Add(time.Second).UnixNano() {
		t.Fatal("old watermark or generation")
	}
	if n.getStack(third) == nil {
		t.Fatal("new packet rejected")
	}
	n.queue.dropPair(canonicalPair(testPair))
	// Endpoint removal and replacement retain policy and generation.
	old := n.endpoints[testPair.DestHost]
	old.stack.endpoint = old
	n.DetachStack(old.stack)
	replacement := &Stack{}
	n.AttachStack(testPair.DestHost, replacement)
	if s := n.links[canonicalPair(testPair)]; s.Generation != 1 || s.Latency == nil {
		t.Fatal("restart lost policy")
	}
	p := sendTestPacket(n, testPair)
	if n.getStack(p) != replacement {
		t.Fatal("replacement not attached")
	}
	n.queue.dropPair(canonicalPair(testPair))
}

func TestNetworkFiveHostIsolation(t *testing.T) {
	rng := &scriptedRandom{}
	n := testNetwork(t, rng)
	hosts := make([]netip.Addr, 5)
	for i := range hosts {
		hosts[i] = netip.AddrFrom4([4]byte{10, 0, 0, byte(i + 1)})
	}
	attachTestEndpoints(n, hosts...)
	// Keep packets queued, including both directions of every unordered pair.
	c := DefaultConfig()
	c.Latency.Min = time.Hour
	c.Latency.Max = time.Hour
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: c})
	target := HostPair{hosts[1], hosts[4]}
	key := canonicalPair(target)
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			rng.exponentials = append(rng.exponentials, 0, 0)
			pair := HostPair{hosts[i], hosts[j]}
			sendTestPacket(n, pair)
			sendTestPacket(n, pair.Flip())
		}
	}
	if len(n.queue.items) != 20 {
		t.Fatal(len(n.queue.items))
	}
	control(t, n, ControlRequest{Kind: SetPairLatency, A: hosts[1].String(), B: hosts[4].String(), Duration: 2 * time.Hour})
	control(t, n, ControlRequest{Kind: SetPairFailRate, A: hosts[1].String(), B: hosts[4].String(), Probability: 1})
	control(t, n, ControlRequest{Kind: SetPairRepairRate, A: hosts[1].String(), B: hosts[4].String(), Probability: 0})
	sendTestPacket(n, target)
	if len(n.queue.items) != 18 || n.links[key].Status != linkRandomPartition {
		t.Fatal("pair failure did not purge exactly two packets")
	}
	for other, s := range n.links {
		if other == key {
			continue
		}
		if s.Latency != nil || s.Loss != nil || s.Status != linkHealthy || s.Generation != 0 || n.effectiveConfigLocked(s) != c {
			t.Fatal("unrelated pair changed", other, s)
		}
	}
	if _, ok := n.endpoints[hosts[1]].last[hosts[4]]; ok {
		t.Fatal("forward watermark retained")
	}
	if _, ok := n.endpoints[hosts[4]].last[hosts[1]]; ok {
		t.Fatal("reverse watermark retained")
	}
	if len(n.endpoints[hosts[1]].last) != 3 {
		t.Fatal("unrelated watermarks cleared")
	}
	// Changing policy does not repair; repair is triggered by a subsequent send.
	control(t, n, ControlRequest{Kind: SetPairFailRate, A: hosts[1].String(), B: hosts[4].String(), Probability: 0})
	if n.links[key].Status != linkRandomPartition {
		t.Fatal("config repaired link")
	}
	sendTestPacket(n, target.Flip())
	if len(n.queue.items) != 18 {
		t.Fatal("repair probability zero ignored")
	}
	control(t, n, ControlRequest{Kind: SetPairRepairRate, A: hosts[1].String(), B: hosts[4].String(), Probability: 1})
	rng.exponentials = append(rng.exponentials, 0)
	sendTestPacket(n, target.Flip())
	if len(n.queue.items) != 19 || n.links[key].Status != linkHealthy {
		t.Fatal("send did not repair")
	}
	for other := range n.links {
		n.queue.dropPair(other)
	}
}

func TestNetworkOneWayNoMutation(t *testing.T) {
	for _, status := range []linkStatus{linkHealthy, linkRandomPartition, linkExplicitPartition} {
		for _, reverse := range []bool{false, true} {
			for _, connected := range []bool{false, true} {
				rng := &scriptedRandom{exponentials: []float64{0, 0}}
				n := testNetwork(t, rng)
				attachTestEndpoints(n, testPair.SourceHost, testPair.DestHost)
				if err := n.SetDelay(testPair, time.Hour); err != nil {
					t.Fatal(err)
				}
				sendTestPacket(n, testPair)
				sendTestPacket(n, testPair.Flip())
				s := n.links[canonicalPair(testPair)]
				s.Status = status
				before := *s
				config := n.config
				sequence := n.nextSequence
				packets := append([]*Packet(nil), n.queue.items...)
				arrival := packets[0].ArrivalTime
				pair := testPair
				if reverse {
					pair = pair.Flip()
				}
				r := ControlRequest{Kind: SetConnectivity, A: pair.SourceHost.String(), B: pair.DestHost.String(), Direction: DirectionOneWay, Connected: connected}
				if n.ApplyControl(r) != syscall.EOPNOTSUPP {
					t.Fatal("one-way was not rejected")
				}
				if !reflect.DeepEqual(before, *s) || n.config != config || n.nextSequence != sequence || !reflect.DeepEqual(packets, []*Packet(n.queue.items)) || packets[0].ArrivalTime != arrival || len(rng.calls) != 2 {
					t.Fatal("unsupported call mutated network")
				}
				if len(n.endpoints[testPair.SourceHost].last) != 1 || len(n.endpoints[testPair.DestHost].last) != 1 {
					t.Fatal("unsupported call reset ordering")
				}
				n.queue.dropPair(canonicalPair(testPair))
			}
		}
	}
	n := testNetwork(t, &scriptedRandom{})
	r := localRequest(SetConnectivity)
	r.Direction = DirectionOneWay
	if n.ApplyControl(r) != syscall.EOPNOTSUPP || len(n.links) != 0 {
		t.Fatal("unsupported call allocated state")
	}
	r.A = "invalid"
	if n.ApplyControl(r) != syscall.EINVAL {
		t.Fatal("invalid address accepted")
	}
	r = localRequest(SetConnectivity)
	r.Direction = 99
	if n.ApplyControl(r) != syscall.EINVAL {
		t.Fatal("unknown direction accepted")
	}
}

func TestNetworkDropsWithoutSampling(t *testing.T) {
	rng := &scriptedRandom{}
	n := testNetwork(t, rng)
	attachTestEndpoints(n, testPair.SourceHost)
	p := sendTestPacket(n, testPair)
	if p.Kind != 255 || len(n.links) != 0 {
		t.Fatal("missing endpoint packet not released")
	}
	n.endpoints[testPair.SourceHost].closed = true
	p = sendTestPacket(n, testPair)
	if p.Kind != 255 {
		t.Fatal("closed endpoint packet not released")
	}
	n.endpoints[testPair.SourceHost].closed = false
	attachTestEndpoints(n, testPair.DestHost)
	if err := n.SetConnected(testPair, false); err != nil {
		t.Fatal(err)
	}
	p = sendTestPacket(n, testPair)
	if p.Kind != 255 || len(rng.calls) != 0 {
		t.Fatal("explicit partition sampled or leaked packet")
	}
}

func TestNetworkCounterExhaustion(t *testing.T) {
	for _, which := range []string{"generation", "sequence"} {
		t.Run(which, func(t *testing.T) {
			n := testNetwork(t, &scriptedRandom{exponentials: []float64{0}})
			attachTestEndpoints(n, testPair.SourceHost, testPair.DestHost)
			defer func() {
				if recover() == nil {
					t.Fatal("counter wrapped")
				}
			}()
			if which == "generation" {
				n.getOrCreateLinkLocked(canonicalPair(testPair)).Generation = math.MaxUint64
				_ = n.SetConnected(testPair, false)
			} else {
				n.nextSequence = math.MaxUint64
				sendTestPacket(n, testPair)
			}
		})
	}
}

func TestNetworkSampledOrderAcrossGlobalUpdates(t *testing.T) {
	rng := &scriptedRandom{exponentials: []float64{100, 0, 0}}
	n := testNetwork(t, rng)
	attachTestEndpoints(n, testPair.SourceHost, testPair.DestHost)
	first := sendTestPacket(n, testPair)
	second := sendTestPacket(n, testPair)
	c := DefaultConfig()
	c.Latency.Max = 0
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: c})
	third := sendTestPacket(n, testPair)
	if first.ArrivalTime.time != second.ArrivalTime.time || first.ArrivalTime.time != third.ArrivalTime.time {
		t.Fatal("sampled/global updates reordered arrivals")
	}
	if first.ArrivalTime.index >= second.ArrivalTime.index || second.ArrivalTime.index >= third.ArrivalTime.index {
		t.Fatal("unstable ties")
	}
	n.queue.dropPair(canonicalPair(testPair))
}

func TestNetworkSelfPair(t *testing.T) {
	rng := &scriptedRandom{exponentials: []float64{0}}
	n := testNetwork(t, rng)
	addr := testPair.SourceHost
	attachTestEndpoints(n, addr)
	pair := HostPair{addr, addr}
	if err := n.SetDelay(pair, time.Hour); err != nil {
		t.Fatal(err)
	}
	sendTestPacket(n, pair)
	if err := n.SetConnected(pair, false); err != nil {
		t.Fatal(err)
	}
	if len(n.queue.items) != 0 || len(n.endpoints[addr].last) != 0 || n.links[canonicalPair(pair)].Generation != 1 {
		t.Fatal("self pair not partitioned exactly once")
	}
}

// Also run under gosim with -simtrace=network to exercise enabled formatting.
func TestTraceDoesNotSample(t *testing.T) {
	rng := &scriptedRandom{}
	n := testNetwork(t, rng)
	r := localRequest(SetPairCurve)
	r.Lambda = 2.5
	control(t, n, r)
	control(t, n, ControlRequest{Kind: SetGlobalFailRate, Probability: 0.125})
	if err := n.SetConnected(testPair, false); err != nil {
		t.Fatal(err)
	}
	if err := n.SetConnected(testPair, true); err != nil {
		t.Fatal(err)
	}
	control(t, n, localRequest(ResetPairLatency))
	if len(rng.calls) != 0 {
		t.Fatal("control/trace formatting consumed RNG", rng.calls)
	}
}
