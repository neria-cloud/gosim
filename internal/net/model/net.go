package model

import (
	"math"
	"net/netip"
	"syscall"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
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
	ref   contracts.EpochRef

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
	drop := func(reason string) {
		n.traceNetwork(
			"drop",
			"from",
			e.addr,
			"to",
			packet.ConnId.Hosts.DestHost,
			"kind",
			packet.Kind,
			"reason",
			reason,
		)
		freePacket(packet)
	}
	if e.closed {
		drop("source_closed")
		return
	}
	packet.SourceEpoch = e.ref
	dst := packet.ConnId.Hosts.DestHost
	if n.endpoints[dst] == nil || n.endpoints[dst].stack.quiesced {
		drop("destination_absent")
		return
	}
	packet.DestinationEpoch = n.endpoints[dst].ref
	key := edgeKey(packet.ConnId.Hosts)
	state := n.getOrCreateLink(key)
	config := n.effectiveConfig(state)
	if !n.advanceLink(key, state, config.Loss) {
		if state.Explicit {
			drop("explicit_partition")
		} else {
			drop("random_partition")
		}
		return
	}
	sampled := sampleLatency(n.rng, config.Latency)
	arrival, saturated := saturatingArrival(n.clock.NowNanos(), sampled)
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
	n.traceNetwork(
		"enqueue",
		"from",
		e.addr,
		"to",
		dst,
		"kind",
		packet.Kind,
		"sampled",
		sampled,
		"arrival",
		arrival,
		"sequence",
		n.nextSequence,
		"generation",
		state.Generation,
		"saturated",
		saturated,
	)
	n.queue.enqueue(packet)
}

type Network struct {
	endpoints                                     map[netip.Addr]*Endpoint
	config                                        Config
	links                                         map[linkKey]*linkState
	rng                                           contracts.Random
	clock                                         contracts.Clock
	events                                        contracts.Events
	trace                                         contracts.Trace
	nextSequence, nextReservation, nextConnection uint64
	queue                                         *delayQueue
	reservations                                  map[uint64]Reservation
	reserved                                      map[netip.Addr]uint64
	assigned                                      map[contracts.MachineRef]netip.Addr
	owners                                        map[netip.Addr]contracts.MachineRef
}

// Reservation is a value token; only Network can publish or retire it.
type Reservation struct {
	id      uint64
	owner   contracts.MachineRef
	address netip.Addr
}

func (r Reservation) Address() netip.Addr { return r.address }

func NewNetwork(
	clock contracts.Clock,
	events contracts.Events,
	random contracts.Random,
	trace contracts.Trace,
) *Network {
	if clock == nil || events == nil || random == nil || trace == nil {
		panic("missing network ports")
	}
	n := &Network{
		endpoints:    make(map[netip.Addr]*Endpoint),
		config:       DefaultConfig(),
		links:        make(map[linkKey]*linkState),
		rng:          random,
		clock:        clock,
		events:       events,
		trace:        trace,
		reservations: make(map[uint64]Reservation),
		reserved:     make(map[netip.Addr]uint64),
		assigned:     make(map[contracts.MachineRef]netip.Addr),
		owners:       make(map[netip.Addr]contracts.MachineRef),
	}
	n.queue = newDelayQueue(clock, events, func(p *Packet) {
		if stack := n.getStack(p); stack != nil {
			stack.ProcessIncoming(p)
		}
		freePacket(p)
	})
	return n
}

func (n *Network) ReserveAddress(owner contracts.MachineRef, raw [4]byte, explicit bool) (Reservation, error) {
	if owner.RunID == 0 || owner.ID == 0 {
		return Reservation{}, syscall.EINVAL
	}
	for _, pending := range n.reservations {
		if pending.owner == owner {
			return Reservation{}, syscall.EADDRINUSE
		}
	}
	addr := netip.AddrFrom4(raw)
	if prior, ok := n.assigned[owner]; ok {
		if explicit && addr != prior {
			return Reservation{}, syscall.EINVAL
		}
		addr = prior
	} else if !explicit {
		addr = netip.AddrFrom4([4]byte{11, 0, 0, 1})
		for n.owners[addr] != (contracts.MachineRef{}) || n.reserved[addr] != 0 {
			addr = addr.Next()
			if !addr.Is4() {
				return Reservation{}, syscall.EADDRNOTAVAIL
			}
		}
	}
	if addr.IsUnspecified() || addr.IsMulticast() {
		return Reservation{}, syscall.EINVAL
	}
	if n.endpoints[addr] != nil || n.reserved[addr] != 0 {
		return Reservation{}, syscall.EADDRINUSE
	}
	if old := n.owners[addr]; old != (contracts.MachineRef{}) && old != owner {
		return Reservation{}, syscall.EADDRINUSE
	}
	if n.nextReservation == math.MaxUint64 {
		panic("network reservation exhausted")
	}
	n.nextReservation++
	r := Reservation{n.nextReservation, owner, addr}
	n.reservations[r.id] = r
	n.reserved[addr] = r.id
	return r, nil
}

func (n *Network) AbortReservation(r Reservation) {
	if n.reservations[r.id] != r || r.id == 0 {
		return
	}
	delete(n.reservations, r.id)
	delete(n.reserved, r.address)
}

func (n *Network) Attach(r Reservation, ref contracts.EpochRef, stack *Stack) {
	if r.id == 0 || n.reservations[r.id] != r || ref.Machine != r.owner || ref.Epoch == 0 || stack == nil ||
		stack.ref != ref ||
		stack.endpoint != nil ||
		stack.shutdown {
		panic("invalid network publication")
	}
	n.AbortReservation(r)
	n.assigned[r.owner] = r.address
	n.owners[r.address] = r.owner
	ep := &Endpoint{net: n, addr: r.address, stack: stack, ref: ref, last: make(map[netip.Addr]timeIndex)}
	n.endpoints[r.address] = ep
	stack.setEndpoint(ep)
}

func (n *Network) Detach(ref contracts.EpochRef) {
	addr := n.assigned[ref.Machine]
	ep := n.endpoints[addr]
	if ep == nil || ep.ref != ref {
		return
	}
	ep.stack.Quiesce()
	ep.closed = true
	ep.stack.setEndpoint(nil)
	ep.stack = nil
	clear(ep.last)
	delete(n.endpoints, addr)
	n.queue.drop(func(p *Packet) bool { return p.DestinationEpoch == ref })
}

func (n *Network) ReleaseAddress(owner contracts.MachineRef) {
	if addr, ok := n.assigned[owner]; ok {
		if n.endpoints[addr] != nil {
			panic("release active address")
		}
		delete(n.owners, addr)
		delete(n.assigned, owner)
	}
	for _, r := range n.reservations {
		if r.owner == owner {
			n.AbortReservation(r)
		}
	}
}

func (n *Network) Close() {
	if len(n.endpoints) != 0 {
		panic("close network with active endpoints")
	}
	n.queue.drop(func(*Packet) bool { return true })
	clear(n.reservations)
	clear(n.reserved)
	clear(n.assigned)
	clear(n.owners)
	clear(n.links)
}

func (n *Network) connectionGeneration() uint64 {
	if n.nextConnection == math.MaxUint64 {
		panic("network connection generation exhausted")
	}
	n.nextConnection++
	return n.nextConnection
}

func (n *Network) SetConnected(p HostPair, connected bool) error {
	return n.SetConnectivity(p, connected, DirectionTwoWay)
}

func (n *Network) SetConnectivity(p HostPair, connected bool, direction ConnectivityDirection) error {
	keys, err := selectedEdges(p, direction)
	if err != nil {
		return err
	}
	// Validate every edge before mutating either half of a symmetric update.
	if !connected {
		for _, key := range keys {
			if s := n.links[key]; s != nil && s.Generation == math.MaxUint64 {
				panic("network generation exhausted")
			}
		}
	}
	for _, key := range keys {
		state := n.getOrCreateLink(key)
		if !connected {
			n.partition(key, state, linkExplicitPartition)
		} else {
			state.Explicit = false
			n.traceNetwork("manual_repair", "a", key.A, "b", key.B, "generation", state.Generation)
		}
	}
	return nil
}

func (n *Network) SetDelay(p HostPair, delay time.Duration) error {
	return n.ApplyControl(
		ControlRequest{Kind: SetPairLatency, A: p.SourceHost.String(), B: p.DestHost.String(), Duration: delay},
	)
}

func (n *Network) getStack(packet *Packet) *Stack {
	state := n.links[edgeKey(packet.ConnId.Hosts)]
	if state == nil || state.Generation != packet.Generation || state.Status != linkHealthy || state.Explicit {
		n.traceNetwork(
			"drop",
			"reason",
			"stale_or_partitioned",
			"from",
			packet.ConnId.Hosts.SourceHost,
			"to",
			packet.ConnId.Hosts.DestHost,
			"generation",
			packet.Generation,
		)
		return nil
	}
	endpoint := n.endpoints[packet.ConnId.Hosts.DestHost]
	if endpoint == nil || endpoint.closed || endpoint.stack == nil || endpoint.ref != packet.DestinationEpoch {
		n.traceNetwork("drop", "reason", "destination_absent", "to", packet.ConnId.Hosts.DestHost)
		return nil
	}
	// Delivery is serialized with topology mutation by the event owner.
	n.traceNetwork(
		"deliver_admitted",
		"from",
		packet.ConnId.Hosts.SourceHost,
		"to",
		packet.ConnId.Hosts.DestHost,
		"kind",
		packet.Kind,
		"sequence",
		packet.ArrivalTime.index,
		"generation",
		packet.Generation,
	)

	return endpoint.stack
}
