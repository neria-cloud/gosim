package model

import (
	"math"
	"net/netip"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func liveNetwork(t *testing.T) (*Network, *testEvents, *Stack, *Stack) {
	t.Helper()
	events := newTestEvents()
	n := NewNetwork(events, events, gosimruntime.NewDomainRandom(42, "net", 0), &testTrace{})
	if err := n.SetDelay(testPair, 0); err != nil {
		t.Fatal(err)
	}
	attachTestEndpoints(n, testPair.SourceHost, testPair.DestHost)
	a, b := n.endpoints[testPair.SourceHost].stack, n.endpoints[testPair.DestHost].stack
	t.Cleanup(func() {
		// Includes replacement epochs, not just the fixture's initial two stacks.
		for _, addr := range []netip.Addr{testPair.SourceHost, testPair.DestHost} {
			if ep := n.endpoints[addr]; ep != nil {
				ep.stack.Shutdown(contracts.StopCrash)
				n.Detach(ep.ref)
			}
		}
		n.Close()
		if len(events.pending) != 0 {
			t.Fatal("network leaked callbacks", len(events.pending))
		}
	})
	return n, events, a, b
}

func connect(t *testing.T, events *testEvents, a, b *Stack) (*Stream, *Stream, *Listener) {
	t.Helper()
	listener, err := b.OpenListener(80)
	if err != nil {
		t.Fatal(err)
	}
	client, err := a.OpenStream(netip.AddrPortFrom(b.endpoint.addr, 80))
	if err != nil {
		t.Fatal(err)
	}
	events.advance(events.now)
	server, err := b.Accept(listener)
	if err != nil {
		t.Fatal(err)
	}
	if !client.opened || !server.opened || client.generation != server.generation {
		t.Fatal("handshake failed")
	}
	return client, server, listener
}

func TestAddressReservationRollbackAndRestart(t *testing.T) {
	rng := &scriptedRandom{}
	n := testNetwork(t, rng)
	owner := contracts.MachineRef{RunID: 1, ID: 1}
	first, err := n.ReserveAddress(owner, [4]byte{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.ReserveAddress(owner, [4]byte{}, false); err != syscall.EADDRINUSE {
		t.Fatal("duplicate owner reservation", err)
	}
	secondOwner := contracts.MachineRef{RunID: 1, ID: 2}
	second, err := n.ReserveAddress(secondOwner, [4]byte{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.address == second.address || first.address.String() != "11.0.0.1" {
		t.Fatal("reservation collision")
	}
	if _, err := n.ReserveAddress(secondOwner, first.address.As4(), true); err != syscall.EADDRINUSE {
		t.Fatal(err)
	}
	n.AbortReservation(first)
	n.AbortReservation(first)
	retry, err := n.ReserveAddress(owner, [4]byte{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if retry.address != first.address || retry.id <= second.id {
		t.Fatal("abort consumed address or reused token")
	}
	ref := contracts.EpochRef{Machine: owner, Epoch: 1}
	stack := NewStack(ref, contracts.Ports{Clock: n.clock, Events: n.events})
	n.Attach(retry, ref, stack)
	n.AbortReservation(retry)
	if n.endpoints[retry.address] == nil {
		t.Fatal("abort retired published endpoint")
	}
	stack.Shutdown(contracts.StopCrash)
	n.Detach(ref)
	if _, err := n.ReserveAddress(secondOwner, retry.address.As4(), true); err != syscall.EADDRINUSE {
		t.Fatal("detached address lost owner", err)
	}
	restart, err := n.ReserveAddress(owner, [4]byte{}, false)
	if err != nil || restart.address != retry.address {
		t.Fatal("restart identity", restart, err)
	}
	n.AbortReservation(restart)
	n.AbortReservation(second)
	n.ReleaseAddress(owner)
	reused, err := n.ReserveAddress(secondOwner, [4]byte{}, false)
	if err != nil || reused.address != retry.address {
		t.Fatal("destroy did not release address")
	}
	n.AbortReservation(reused)
	n.Close()
	if len(rng.calls) != 0 || len(n.events.(*testEvents).pending) != 0 {
		t.Fatal("preparation sampled or scheduled")
	}
}

func TestStalePacketsAfterRestartAndPortReuse(t *testing.T) {
	n, events, a, b := liveNetwork(t)
	client, server, _ := connect(t, events, a, b)
	stale := Packet{ConnId: client.id, Kind: PacketKindStreamClose, SourceEpoch: a.ref, DestinationEpoch: b.ref, ConnectionGeneration: client.generation}
	if err := a.StreamClose(client); err != nil {
		t.Fatal(err)
	}
	events.advance(events.now)
	if !server.closed {
		t.Fatal("close not delivered")
	}
	a.nextPort = uint32(client.id.Ports.SourcePort) // force same-epoch tuple reuse
	next, err := a.OpenStream(netip.AddrPortFrom(b.endpoint.addr, 80))
	if err != nil {
		t.Fatal(err)
	}
	events.advance(events.now)
	replacement := b.streams[next.id.Flip()]
	if replacement == nil || replacement.generation == server.generation {
		t.Fatal("connection generation reused")
	}
	b.ProcessIncoming(&stale)
	if replacement.closed {
		t.Fatal("stale close killed recycled connection")
	}
	oldOpen := Packet{ConnId: client.id, Kind: PacketKindOpenStream, SourceEpoch: a.ref, DestinationEpoch: b.ref, ConnectionGeneration: client.generation, InBuffer: newCircularBuffer(1024), OutBuffer: newCircularBuffer(1024)}
	b.ProcessIncoming(&oldOpen)
	if b.streams[next.id.Flip()] != replacement {
		t.Fatal("stale handshake replaced connection")
	}
	oldRef := b.ref
	b.Shutdown(contracts.StopCrash)
	n.Detach(oldRef)
	r, err := n.ReserveAddress(oldRef.Machine, [4]byte{}, false)
	if err != nil {
		t.Fatal(err)
	}
	newRef := oldRef
	newRef.Epoch++
	fresh := NewStack(newRef, b.ports)
	n.Attach(r, newRef, fresh)
	if _, err := fresh.OpenListener(80); err != nil {
		t.Fatal(err)
	}
	fresh.ProcessIncoming(&oldOpen)
	if len(fresh.streams) != 0 || n.getStack(&stale) != nil {
		t.Fatal("old destination epoch admitted")
	}
	n.Detach(oldRef)
	if n.endpoints[r.address].stack != fresh {
		t.Fatal("stale detach removed replacement")
	}
	// An old source handshake also cannot attach to the new destination.
	oldOpen.DestinationEpoch = newRef
	oldOpen.SourceEpoch.Epoch++
	fresh.ProcessIncoming(&oldOpen)
	if len(fresh.streams) != 0 {
		t.Fatal("old source admitted")
	}
}

func TestGracefulFinalCloseAndCrashSilence(t *testing.T) {
	for _, kind := range []contracts.StopKind{contracts.StopGraceful, contracts.StopCrash} {
		t.Run(map[contracts.StopKind]string{contracts.StopGraceful: "graceful", contracts.StopCrash: "crash"}[kind], func(t *testing.T) {
			n, events, a, b := liveNetwork(t)
			client, server, _ := connect(t, events, a, b)
			if err := n.SetDelay(testPair, time.Second); err != nil {
				t.Fatal(err)
			}
			staleCallback := events.pending[server.keepaliveTimer.event].fn
			// Mirrors kernel ordering: epoch events canceled before provider quiescence.
			events.CancelEpoch(b.ref)
			b.Quiesce()
			b.Shutdown(kind)
			n.Detach(b.ref)
			if len(b.streams) != 0 || len(b.listeners) != 0 || server.inBuffer != nil || server.outBuffer != nil {
				t.Fatal("retained stack resources")
			}
			for _, event := range events.pending {
				if event.ref == b.ref {
					t.Fatal("event targets canceled source")
				}
			}
			want := 0
			if kind == contracts.StopGraceful {
				want = 1
			}
			if len(n.queue.items) != want {
				t.Fatal("wrong final notification count", len(n.queue.items))
			}
			if want == 1 {
				p := n.queue.items[0]
				if p.Kind != PacketKindStreamClose || p.InBuffer != nil || p.OutBuffer != nil || p.DestinationEpoch != a.ref || p.ConnectionGeneration != client.generation {
					t.Fatal("final close captured mutable source")
				}
			}
			staleCallback()
			events.advance(int64(2 * time.Second))
			if client.closed != (kind == contracts.StopGraceful) {
				t.Fatal("peer close does not match stop kind")
			}
			if _, err := b.OpenStream(netip.AddrPortFrom(a.endpoint.addr, 80)); err != ErrStreamClosed {
				t.Fatal("quiesced application work admitted")
			}
		})
	}
}

func TestQuiesceRearmsQueueForSurvivingDestination(t *testing.T) {
	n, events, a, b := liveNetwork(t)
	if err := n.SetDelay(testPair, time.Second); err != nil {
		t.Fatal(err)
	}
	first := sendTestPacket(n, testPair)
	second := sendTestPacket(n, testPair.Flip())
	if events.pending[n.queue.event].ref != b.ref {
		t.Fatal("unexpected head")
	}
	events.CancelEpoch(b.ref)
	b.Quiesce()
	if first.Kind != 255 || len(n.queue.items) != 1 || n.queue.items[0] != second || events.pending[n.queue.event].ref != a.ref {
		t.Fatal("stopped destination stranded unrelated packet")
	}
	events.advance(int64(time.Second))
	if len(n.queue.items) != 0 {
		t.Fatal("survivor not delivered")
	}
}

func TestListenerCloseDrainsWholeBacklog(t *testing.T) {
	_, events, a, b := liveNetwork(t)
	listener, err := b.OpenListener(80)
	if err != nil {
		t.Fatal(err)
	}
	var clients []*Stream
	for range 5 {
		stream, err := a.OpenStream(netip.AddrPortFrom(b.endpoint.addr, 80))
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, stream)
	}
	events.advance(events.now)
	if listener.incomingCount != 5 {
		t.Fatal("backlog not full")
	}
	if err := b.CloseListener(listener); err != nil {
		t.Fatal(err)
	}
	events.advance(events.now)
	if len(b.streams) != 0 || listener.incomingCount != 0 {
		t.Fatal("partial backlog cleanup")
	}
	for _, stream := range clients {
		if !stream.closed {
			t.Fatal("pending peer not closed")
		}
	}
}

type readinessRecorder struct {
	refs  []contracts.EpochRef
	masks []contracts.Readiness
}

func (r *readinessRecorder) Notify(ref contracts.EpochRef, _ int, mask contracts.Readiness) {
	r.refs = append(r.refs, ref)
	r.masks = append(r.masks, mask)
}

func TestStreamDataPollingAndTimerRetirement(t *testing.T) {
	_, events, a, b := liveNetwork(t)
	client, server, _ := connect(t, events, a, b)
	sink := &readinessRecorder{}
	b.RegisterStreamPoller(sink, server)
	stale := events.pending[server.keepaliveTimer.event].fn
	payload := make([]byte, 1400)
	for i := range payload {
		payload[i] = byte(i)
	}
	count, err := a.StreamSend(client, abi.ByteView{Ptr: payload})
	if count != 1024 || err != nil {
		t.Fatal(count, err)
	}
	if _, err := a.StreamSend(client, abi.ByteView{Ptr: payload}); err != syscall.EWOULDBLOCK {
		t.Fatal("full window", err)
	}
	events.advance(events.now)
	result := make([]byte, 700)
	count, err = b.StreamRecv(server, abi.ByteView{Ptr: result})
	if count != 700 || err != nil || !reflect.DeepEqual(result, payload[:700]) {
		t.Fatal("first read", count, err)
	}
	events.advance(events.now)
	count, err = a.StreamSend(client, abi.ByteView{Ptr: payload[1024:]})
	if count != 376 || err != nil {
		t.Fatal("wrap write", count, err)
	}
	events.advance(events.now)
	count, err = b.StreamRecv(server, abi.ByteView{Ptr: result})
	if count != 700 || err != nil || !reflect.DeepEqual(result, payload[700:]) {
		t.Fatal("wrap read", count, err)
	}
	events.advance(events.now)
	if len(sink.masks) < 3 || sink.masks[1]&contracts.ReadReady == 0 || sink.masks[len(sink.masks)-1]&contracts.ReadReady != 0 {
		t.Fatal("readiness", sink.masks)
	}
	for _, ref := range sink.refs {
		if ref != b.ref {
			t.Fatal("wrong readiness epoch")
		}
	}
	b.DeregisterStreamPoller(sink, server)
	before := len(sink.masks)
	if err := b.StreamClose(server); err != nil {
		t.Fatal(err)
	}
	stale()
	events.advance(events.now)
	if !client.closed || len(sink.masks) != before {
		t.Fatal("retired poller or timer fired")
	}
}

func TestDirectedPolicyCopiesAndAtomicUpdates(t *testing.T) {
	n := testNetwork(t, &scriptedRandom{})
	latency := LatencyConfig{Min: 10, Max: 20, Lambda: 3}
	loss := LossConfig{FailProbability: .2, RepairProbability: .3}
	if err := n.SetLinkPolicy(testPair, DirectionOneWay, &latency, &loss); err != nil {
		t.Fatal(err)
	}
	forward := n.links[edgeKey(testPair)]
	latency.Max = 99
	loss.FailProbability = .9
	if forward.Latency.Max != 20 || forward.Loss.FailProbability != .2 || n.links[edgeKey(testPair.Flip())] != nil {
		t.Fatal("alias or reverse update")
	}
	before := n.effectiveConfig(forward)
	bad := LossConfig{FailProbability: math.NaN()}
	if n.SetLinkPolicy(testPair, DirectionTwoWay, &latency, &bad) != syscall.EINVAL || n.effectiveConfig(forward) != before || len(n.links) != 1 {
		t.Fatal("invalid grouped update partially applied")
	}
	if err := n.SetLinkPolicy(testPair, DirectionTwoWay, nil, &loss); err != nil {
		t.Fatal(err)
	}
	reverse := n.links[edgeKey(testPair.Flip())]
	if forward.Latency.Max != 20 || reverse.Latency != nil || forward.Loss == reverse.Loss {
		t.Fatal("nil changed group or shared snapshot")
	}
	if err := n.ResetLinkPolicy(testPair, DirectionOneWay); err != nil {
		t.Fatal(err)
	}
	if forward.Latency != nil || forward.Loss != nil || reverse.Loss == nil {
		t.Fatal("directional reset")
	}
}

func TestNetworkRandomStreamIndependence(t *testing.T) {
	run := func(noise bool) []timeIndex {
		events := newTestEvents()
		rng := gosimruntime.NewDomainRandom(19, "net", 0)
		n := NewNetwork(events, events, rng, &testTrace{})
		attachTestEndpoints(n, testPair.SourceHost, testPair.DestHost)
		var got []timeIndex
		for range 20 {
			if noise {
				for _, domain := range []string{"fs", "application", "scheduler"} {
					r := gosimruntime.NewDomainRandom(19, domain, 0)
					for range 50 {
						r.Uint64()
					}
				}
			}
			got = append(got, sendTestPacket(n, testPair).ArrivalTime)
		}
		n.queue.drop(func(*Packet) bool { return true })
		return got
	}
	if !reflect.DeepEqual(run(false), run(true)) {
		t.Fatal("unrelated random domain changed packets")
	}
}

func TestSymmetricFieldUpdateFailsWithoutMutationOrTrace(t *testing.T) {
	n := testNetwork(t, &scriptedRandom{})
	latency := LatencyConfig{Min: 10, Max: 20, Lambda: 1}
	if err := n.SetLinkPolicy(testPair.Flip(), DirectionOneWay, &latency, nil); err != nil {
		t.Fatal(err)
	}
	trace := n.trace.(*testTrace)
	before := len(trace.names)
	err := n.ApplyControl(ControlRequest{Kind: SetPairMaxLatency, A: testPair.SourceHost.String(), B: testPair.DestHost.String(), Duration: 5})
	if err != syscall.EINVAL || len(n.links) != 1 || len(trace.names) != before || n.links[edgeKey(testPair.Flip())].Latency.Max != 20 {
		t.Fatal("partially published invalid Both update")
	}
}

func TestDetachRetiresStackEvents(t *testing.T) {
	n, events, a, b := liveNetwork(t)
	_, server, _ := connect(t, events, a, b)
	stale := events.pending[server.keepaliveTimer.event].fn
	n.Detach(b.ref)
	stale()
	for _, event := range events.pending {
		if event.ref == b.ref {
			t.Fatal("detached epoch retained callback")
		}
	}
	if !b.quiesced {
		t.Fatal("detached stack accepts work")
	}
}
