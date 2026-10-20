package model

import (
	"math"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
)

const (
	networkHostCount       = 5
	networkLastHost        = 4
	networkPacketCount     = 20
	networkAfterDropCount  = 18
	networkOtherWatermarks = 3
	networkFreedPacketKind = 255
	networkSampleValue     = 100
	networkCurve           = 2.5
)

func attachTestEndpoints(n *Network, hosts ...netip.Addr) {
	for _, addr := range hosts {
		ref := contracts.EpochRef{Machine: contracts.MachineRef{RunID: 1, ID: uint64(addr.As4()[3])}, Epoch: 1}
		r, err := n.ReserveAddress(ref.Machine, addr.As4(), true)
		if err != nil {
			panic(err)
		}
		stack := NewStack(ref, contracts.Ports{Clock: n.clock, Events: n.events, Trace: n.trace})
		n.Attach(r, ref, stack)
	}
}

func sendTestPacket(n *Network, pair HostPair) *Packet {
	p := allocPacket(PacketKindStreamData, ConnId{Hosts: pair})
	n.endpoints[pair.SourceHost].Send(p)
	return p
}

func TestNetworkPartitionAdmissionAndOrdering(t *testing.T) {
	t.Parallel()
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
	n.queue.items = nil
	n.queue.updateTimer()
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
	n.queue.dropPair(edgeKey(testPair))
	// Endpoint removal and replacement retain policy and generation.
	old := n.endpoints[testPair.DestHost]
	old.stack.endpoint = old
	n.Detach(old.ref)
	ref := old.ref
	ref.Epoch++
	reservation, err := n.ReserveAddress(ref.Machine, [4]byte{}, false)
	if err != nil {
		t.Fatal(err)
	}
	replacement := NewStack(ref, contracts.Ports{Clock: n.clock, Events: n.events})
	n.Attach(reservation, ref, replacement)
	if s := n.links[edgeKey(testPair)]; s.Generation != 1 || s.Latency == nil {
		t.Fatal("restart lost policy")
	}
	p := sendTestPacket(n, testPair)
	if n.getStack(p) != replacement {
		t.Fatal("replacement not attached")
	}
	n.queue.dropPair(edgeKey(testPair))
}

func TestNetworkFiveHostIsolation(t *testing.T) {
	t.Parallel()
	rng := &scriptedRandom{}
	n := testNetwork(t, rng)
	hosts := make([]netip.Addr, networkHostCount)
	for i := range hosts {
		hosts[i] = netip.AddrFrom4([4]byte{10, 0, 0, byte(i + 1)})
	}
	attachTestEndpoints(n, hosts...)
	// Keep packets queued, including both directions of every unordered pair.
	c := DefaultConfig()
	c.Latency.Min = time.Hour
	c.Latency.Max = time.Hour
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: c})
	target := HostPair{hosts[1], hosts[networkLastHost]}
	key := edgeKey(target)
	for i := 0; i < networkHostCount; i++ {
		for j := i + 1; j < networkHostCount; j++ {
			rng.exponentials = append(rng.exponentials, 0, 0)
			pair := HostPair{hosts[i], hosts[j]}
			sendTestPacket(n, pair)
			sendTestPacket(n, pair.Flip())
		}
	}
	if len(n.queue.items) != networkPacketCount {
		t.Fatal(len(n.queue.items))
	}
	control(
		t,
		n,
		ControlRequest{
			Kind:     SetPairLatency,
			A:        hosts[1].String(),
			B:        hosts[networkLastHost].String(),
			Duration: 2 * time.Hour,
		},
	)
	control(
		t,
		n,
		ControlRequest{Kind: SetPairFailRate, A: hosts[1].String(), B: hosts[networkLastHost].String(), Probability: 1},
	)
	control(
		t,
		n,
		ControlRequest{
			Kind:        SetPairRepairRate,
			A:           hosts[1].String(),
			B:           hosts[networkLastHost].String(),
			Probability: 0,
		},
	)
	sendTestPacket(n, target)
	sendTestPacket(n, target.Flip())
	if len(n.queue.items) != networkAfterDropCount || n.links[key].Status != linkRandomPartition {
		t.Fatal("pair failure did not purge exactly two packets")
	}
	for other, s := range n.links {
		if other == key || other == edgeKey(target.Flip()) {
			continue
		}
		if s.Latency != nil || s.Loss != nil || s.Status != linkHealthy || s.Generation != 0 ||
			n.effectiveConfig(s) != c {
			t.Fatal("unrelated pair changed", other, s)
		}
	}
	if _, ok := n.endpoints[hosts[1]].last[hosts[networkLastHost]]; ok {
		t.Fatal("forward watermark retained")
	}
	if _, ok := n.endpoints[hosts[networkLastHost]].last[hosts[1]]; ok {
		t.Fatal("reverse watermark retained")
	}
	if len(n.endpoints[hosts[1]].last) != networkOtherWatermarks {
		t.Fatal("unrelated watermarks cleared")
	}
	// Changing policy does not repair; repair is triggered by a subsequent send.
	control(
		t,
		n,
		ControlRequest{Kind: SetPairFailRate, A: hosts[1].String(), B: hosts[networkLastHost].String(), Probability: 0},
	)
	if n.links[key].Status != linkRandomPartition {
		t.Fatal("config repaired link")
	}
	sendTestPacket(n, target.Flip())
	if len(n.queue.items) != networkAfterDropCount {
		t.Fatal("repair probability zero ignored")
	}
	control(
		t,
		n,
		ControlRequest{
			Kind:        SetPairRepairRate,
			A:           hosts[1].String(),
			B:           hosts[networkLastHost].String(),
			Probability: 1,
		},
	)
	rng.exponentials = append(rng.exponentials, 0)
	sendTestPacket(n, target.Flip())
	if len(n.queue.items) != 19 || n.links[edgeKey(target.Flip())].Status != linkHealthy ||
		n.links[key].Status != linkRandomPartition {
		t.Fatal("send did not repair")
	}
	for other := range n.links {
		n.queue.dropPair(other)
	}
}

func TestNetworkDirectedConnectivity(t *testing.T) {
	t.Parallel()
	for _, reverse := range []bool{false, true} {
		rng := &scriptedRandom{exponentials: []float64{0, 0}}
		n := testNetwork(t, rng)
		attachTestEndpoints(n, testPair.SourceHost, testPair.DestHost)
		if err := n.SetDelay(testPair, time.Hour); err != nil {
			t.Fatal(err)
		}
		pair := testPair
		if reverse {
			pair = pair.Flip()
		}
		forward := sendTestPacket(n, pair)
		backward := sendTestPacket(n, pair.Flip())
		oldReverse := *n.links[edgeKey(pair.Flip())]
		if err := n.SetConnectivity(pair, false, DirectionOneWay); err != nil {
			t.Fatal(err)
		}
		if forward.Kind != networkFreedPacketKind || len(n.queue.items) != 1 || n.queue.items[0] != backward ||
			*n.links[edgeKey(pair.Flip())] != oldReverse {
			t.Fatal("forward partition affected reverse")
		}
		if _, ok := n.endpoints[pair.SourceHost].last[pair.DestHost]; ok {
			t.Fatal("forward ordering survived")
		}
		if len(n.endpoints[pair.DestHost].last) != 1 {
			t.Fatal("reverse ordering cleared")
		}
		forwardState := n.links[edgeKey(pair)]
		forwardState.Status = linkRandomPartition
		if err := n.SetConnectivity(pair, true, DirectionOneWay); err != nil {
			t.Fatal(err)
		}
		if forwardState.Explicit || forwardState.Status != linkRandomPartition || len(rng.calls) != 2 {
			t.Fatal("explicit repair changed random state or sampled")
		}
		if err := n.SetConnected(pair, false); err != nil {
			t.Fatal(err)
		}
		if len(n.queue.items) != 0 || !n.links[edgeKey(pair.Flip())].Explicit {
			t.Fatal("Both did not partition reverse")
		}
		if err := n.SetConnectivity(pair, true, DirectionOneWay); err != nil {
			t.Fatal(err)
		}
		if !n.links[edgeKey(pair.Flip())].Explicit {
			t.Fatal("forward repair repaired reverse")
		}
	}
	n := testNetwork(t, &scriptedRandom{})
	if n.SetConnectivity(HostPair{}, false, DirectionOneWay) != syscall.EINVAL ||
		n.SetConnectivity(testPair, false, 99) != syscall.EINVAL ||
		len(n.links) != 0 {
		t.Fatal("invalid direction/address mutated state")
	}
}

func TestNetworkDropsWithoutSampling(t *testing.T) {
	t.Parallel()
	rng := &scriptedRandom{}
	n := testNetwork(t, rng)
	attachTestEndpoints(n, testPair.SourceHost)
	p := sendTestPacket(n, testPair)
	if p.Kind != networkFreedPacketKind || len(n.links) != 0 {
		t.Fatal("missing endpoint packet not released")
	}
	n.endpoints[testPair.SourceHost].closed = true
	p = sendTestPacket(n, testPair)
	if p.Kind != networkFreedPacketKind {
		t.Fatal("closed endpoint packet not released")
	}
	n.endpoints[testPair.SourceHost].closed = false
	attachTestEndpoints(n, testPair.DestHost)
	if err := n.SetConnected(testPair, false); err != nil {
		t.Fatal(err)
	}
	p = sendTestPacket(n, testPair)
	if p.Kind != networkFreedPacketKind || len(rng.calls) != 0 {
		t.Fatal("explicit partition sampled or leaked packet")
	}
}

func TestNetworkCounterExhaustion(t *testing.T) {
	t.Parallel()
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
				n.getOrCreateLink(edgeKey(testPair)).Generation = math.MaxUint64
				_ = n.SetConnected(testPair, false)
			} else {
				n.nextSequence = math.MaxUint64
				sendTestPacket(n, testPair)
			}
		})
	}
}

func TestNetworkSampledOrderAcrossGlobalUpdates(t *testing.T) {
	t.Parallel()
	rng := &scriptedRandom{exponentials: []float64{networkSampleValue, 0, 0}}
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
	n.queue.dropPair(edgeKey(testPair))
}

func TestNetworkSelfPair(t *testing.T) {
	t.Parallel()
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
	if len(n.queue.items) != 0 || len(n.endpoints[addr].last) != 0 || n.links[edgeKey(pair)].Generation != 1 {
		t.Fatal("self pair not partitioned exactly once")
	}
}

// Also run under gosim with -simtrace=network to exercise enabled formatting.
func TestTraceDoesNotSample(t *testing.T) {
	t.Parallel()
	rng := &scriptedRandom{}
	n := testNetwork(t, rng)
	r := localRequest(SetPairCurve)
	r.Lambda = networkCurve
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
