package network

import (
	"math"
	"net/netip"
	"sync"
	"syscall"
	"time"
)

type HostPair struct {
	SourceHost, DestHost netip.Addr
}

func (id HostPair) Flip() HostPair {
	return HostPair{SourceHost: id.DestHost, DestHost: id.SourceHost}
}

type PortPair struct {
	SourcePort uint16
	DestPort   uint16
}

func (id PortPair) Flip() PortPair {
	return PortPair{
		SourcePort: id.DestPort,
		DestPort:   id.SourcePort,
	}
}

type ConnId struct {
	Hosts HostPair
	Ports PortPair
}

func (id ConnId) Source() netip.AddrPort {
	return netip.AddrPortFrom(id.Hosts.SourceHost, id.Ports.SourcePort)
}

func (id ConnId) Dest() netip.AddrPort {
	return netip.AddrPortFrom(id.Hosts.DestHost, id.Ports.DestPort)
}

func (id ConnId) Flip() ConnId {
	return ConnId{
		Hosts: id.Hosts.Flip(),
		Ports: id.Ports.Flip(),
	}
}

type Endpoint struct {
	net   *Network
	addr  netip.Addr
	stack *Stack

	// XXX: make this reorder prevention happen at the conn ID level?
	// XXX: make the stack take care of reordering?
	last map[netip.Addr]timeIndex

	closed bool
}

func (e *Endpoint) Addr() netip.Addr {
	return e.addr
}

// Send owns packet and either frees it or transfers it to the delay queue.
func (e *Endpoint) Send(packet *Packet) {
	if packet.ConnId.Hosts.SourceHost != e.addr {
		panic("bad packet")
	}
	n := e.net
	n.mu.Lock()
	defer n.mu.Unlock()
	drop := func(reason string) {
		traceNetwork("drop", "from", e.addr, "to", packet.ConnId.Hosts.DestHost, "kind", packet.Kind, "reason", reason)
		freePacket(packet)
	}
	if e.closed {
		drop("source_closed")
		return
	}
	dst := packet.ConnId.Hosts.DestHost
	if n.endpoints[dst] == nil {
		drop("destination_absent")
		return
	}
	key := canonicalPair(packet.ConnId.Hosts)
	state := n.getOrCreateLinkLocked(key)
	config := n.effectiveConfigLocked(state)
	if !n.advanceLinkLocked(key, state, config.Loss) {
		if state.Status == linkExplicitPartition {
			drop("explicit_partition")
		} else {
			drop("random_partition")
		}
		return
	}
	sampled := sampleLatency(n.rng, config.Latency)
	arrival, saturated := saturatingArrival(time.Now().UnixNano(), sampled)
	if last, exists := e.last[dst]; exists && last.time > arrival {
		arrival = last.time
	}
	if n.nextSequence == math.MaxUint64 {
		panic("network sequence exhausted")
	}
	n.nextSequence++
	packet.ArrivalTime = timeIndex{time: arrival, index: n.nextSequence}
	packet.Generation = state.Generation
	e.last[dst] = packet.ArrivalTime
	traceNetwork("enqueue", "from", e.addr, "to", dst, "kind", packet.Kind, "sampled", sampled, "arrival", arrival, "sequence", n.nextSequence, "generation", state.Generation, "saturated", saturated)
	n.queue.enqueue(packet)
}

type Network struct {
	mu           sync.Mutex
	endpoints    map[netip.Addr]*Endpoint
	nextIp       netip.Addr
	config       Config
	links        map[linkKey]*linkState
	rng          randomSource
	nextSequence uint64
	queue        *delayQueue
}

func NewNetwork() *Network { return newNetwork(runtimeRandom{}) }

func newNetwork(rng randomSource) *Network {
	if rng == nil {
		panic("nil network random source")
	}
	return &Network{
		endpoints: make(map[netip.Addr]*Endpoint),
		nextIp:    netip.AddrFrom4([4]byte{11, 0, 0, 1}),
		config:    DefaultConfig(), links: make(map[linkKey]*linkState),
		rng: rng, queue: newDelayQueue(),
	}
}

func (n *Network) SetConnected(p HostPair, connected bool) error {
	return n.SetConnectivity(p, connected, DirectionTwoWay)
}

func (n *Network) SetConnectivity(p HostPair, connected bool, direction ConnectivityDirection) error {
	if !p.SourceHost.Is4() || !p.DestHost.Is4() {
		return syscall.EINVAL
	}
	switch direction {
	case DirectionTwoWay:
	case DirectionOneWay:
		// NotImplemented: requires directional state, generations and queue purging.
		return syscall.EOPNOTSUPP
	default:
		return syscall.EINVAL
	}
	key := canonicalPair(p)
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.getOrCreateLinkLocked(key)
	if !connected {
		n.partitionLocked(key, state, linkExplicitPartition)
	} else {
		state.Status = linkHealthy
		traceNetwork("manual_repair", "a", key.A, "b", key.B, "generation", state.Generation)
	}
	return nil
}

func (n *Network) SetDelay(p HostPair, delay time.Duration) error {
	return n.ApplyControl(ControlRequest{Kind: SetPairLatency, A: p.SourceHost.String(), B: p.DestHost.String(), Duration: delay})
}

func (n *Network) NextIP() netip.Addr {
	n.mu.Lock()
	defer n.mu.Unlock()

	addr := n.nextIp
	n.nextIp = n.nextIp.Next()

	return addr
}

func (n *Network) AttachStack(addr netip.Addr, stack *Stack) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, ok := n.endpoints[addr]; ok {
		panic(addr)
	}

	ep := &Endpoint{
		net:   n,
		addr:  addr,
		stack: stack,

		last: make(map[netip.Addr]timeIndex),
	}

	n.endpoints[addr] = ep
	stack.setEndpoint(ep)
}

func (n *Network) DetachStack(stack *Stack) {
	n.mu.Lock()
	defer n.mu.Unlock()

	endpoint, ok := n.endpoints[stack.endpoint.addr]
	if !ok {
		panic(stack.endpoint.addr)
	}

	endpoint.closed = true
	delete(n.endpoints, stack.endpoint.addr)
	stack.setEndpoint(nil) // XXX: good idea?
}

func (n *Network) getStack(packet *Packet) *Stack {
	n.mu.Lock()
	defer n.mu.Unlock()

	state := n.links[canonicalPair(packet.ConnId.Hosts)]
	if state == nil || state.Generation != packet.Generation || state.Status != linkHealthy {
		traceNetwork("drop", "reason", "stale_or_partitioned", "from", packet.ConnId.Hosts.SourceHost, "to", packet.ConnId.Hosts.DestHost, "generation", packet.Generation)
		return nil
	}
	endpoint := n.endpoints[packet.ConnId.Hosts.DestHost]
	if endpoint == nil || endpoint.closed || endpoint.stack == nil {
		traceNetwork("drop", "reason", "destination_absent", "to", packet.ConnId.Hosts.DestHost)
		return nil
	}
	// Admission linearizes here. Never acquire a stack lock while holding n.mu.
	traceNetwork("deliver_admitted", "from", packet.ConnId.Hosts.SourceHost, "to", packet.ConnId.Hosts.DestHost, "kind", packet.Kind, "sequence", packet.ArrivalTime.index, "generation", packet.Generation)

	return endpoint.stack
}

func (n *Network) Run() {
	for {
		packet := n.queue.dequeue()
		if stack := n.getStack(packet); stack != nil {
			// call stack without holding n.mu so stack can send packets which
			// grabs n.mu
			stack.processIncomingPacket(packet)
		}
		freePacket(packet)
	}
}
