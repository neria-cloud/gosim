package model

import (
	"errors"
	"math"
	"net/netip"
	"sort"
	"syscall"
	"time"

	"github.com/neria-cloud/gosim/rt/runtime/abi"

	"github.com/neria-cloud/gosim/internal/contracts"
)

type PacketKind byte

const (
	PacketKindOpenStream   PacketKind = iota // uses InBuffer, OutBuffer
	PacketKindOpenedStream                   // uses nothing
	PacketKindStreamClose                    // uses nothing
	PacketKindStreamData                     // uses Offset, Count
	PacketKindStreamAck                      // uses Offset
	PacketKindStreamWindow                   // uses Count
)

type Packet struct {
	ConnId                        ConnId
	Kind                          PacketKind
	InBuffer, OutBuffer           *circularBuffer
	Offset, Count                 int
	ArrivalTime                   timeIndex // used by delayQueue
	Generation                    uint64
	SourceEpoch, DestinationEpoch contracts.EpochRef
	ConnectionGeneration          uint64
}

func allocPacket(kind PacketKind, id ConnId) *Packet { return &Packet{Kind: kind, ConnId: id} }
func freePacket(p *Packet)                           { *p = Packet{Kind: 255, Offset: -1, Count: -1} }

type Stack struct {
	ref      contracts.EpochRef
	ports    contracts.Ports
	quiesced bool

	shutdown bool

	streams   map[ConnId]*Stream
	seen      map[ConnId]uint64
	listeners map[uint16]*Listener

	endpoint *Endpoint

	nextPort uint32
}

// Quiesce stops application timers/readiness before descriptors are finalized.
func (s *Stack) Quiesce() {
	if s.quiesced {
		return
	}
	s.quiesced = true
	if s.endpoint != nil {
		s.endpoint.net.queue.drop(func(p *Packet) bool { return p.DestinationEpoch == s.ref })
	}
	for _, stream := range s.streams {
		stream.stopTimers()
		stream.pollers.clear()
	}
	for _, listener := range s.listeners {
		listener.pollers.clear()
	}
}

func (s *Stack) Shutdown(kind contracts.StopKind) {
	if s.shutdown {
		return
	}
	if kind != contracts.StopGraceful && kind != contracts.StopCrash {
		panic("invalid network stop kind")
	}
	s.Quiesce()
	// Generation order is deterministic even when descriptor closure did not run.
	streams := make([]*Stream, 0, len(s.streams))
	for _, stream := range s.streams {
		streams = append(streams, stream)
	}
	sort.Slice(streams, func(i, j int) bool { return streams[i].generation < streams[j].generation })
	for _, stream := range streams {
		_ = s.closeStream(stream, kind == contracts.StopGraceful)
		stream.inBuffer = nil
		stream.outBuffer = nil
	}
	for _, listener := range s.listeners {
		listener.closed = true
		clear(listener.incoming)
		listener.incoming = nil
		listener.incomingCount = 0
	}
	s.shutdown = true
	clear(s.streams)
	clear(s.seen)
	clear(s.listeners)
}

func (s *Stream) stopTimers() {
	if s.openFailedTimer != nil {
		s.openFailedTimer.Stop()
		s.openFailedTimer = nil
	}
	if s.ackFailedTimer != nil {
		s.ackFailedTimer.Stop()
		s.ackFailedTimer = nil
	}
	if s.keepaliveTimer != nil {
		s.keepaliveTimer.Stop()
		s.keepaliveTimer = nil
	}
}

func (s *Stack) Endpoint() *Endpoint {
	return s.endpoint
}

func (s *Stack) setEndpoint(ep *Endpoint) {
	s.endpoint = ep
}

func NewStack(ref contracts.EpochRef, ports contracts.Ports) *Stack {
	if ref.Machine.RunID == 0 || ref.Machine.ID == 0 || ref.Epoch == 0 || ports.Clock == nil || ports.Events == nil {
		panic("invalid network stack ports")
	}
	b := &Stack{
		ref: ref, ports: ports,
		streams:   make(map[ConnId]*Stream),
		seen:      make(map[ConnId]uint64),
		listeners: make(map[uint16]*Listener),

		nextPort: 10000,
	}
	return b
}

func (s *Stack) allocatePort() uint16 {
	if s.nextPort > math.MaxUint16 {
		panic("network port space exhausted")
	}
	port := uint16(s.nextPort)
	s.nextPort++
	return port
}

// ProcessIncoming rejects retired destinations and recycled connection tuples.
func (s *Stack) ProcessIncoming(packet *Packet) {
	if s.shutdown || s.quiesced || packet.DestinationEpoch != s.ref || packet.ConnectionGeneration == 0 {
		return
	}
	id := packet.ConnId.Flip()
	if packet.Kind == PacketKindOpenStream {
		source := s.endpoint.net.endpoints[packet.ConnId.Hosts.SourceHost]
		if source == nil || source.ref != packet.SourceEpoch || packet.InBuffer == nil || packet.OutBuffer == nil {
			return
		}
		if packet.ConnectionGeneration <= s.seen[id] {
			return
		}
		s.seen[id] = packet.ConnectionGeneration
		if old := s.streams[id]; old != nil {
			_ = s.closeStream(old, false)
		}
		listener := s.listeners[packet.ConnId.Ports.DestPort]
		if listener == nil || listener.closed || listener.incomingCount == len(listener.incoming) {
			response := allocPacket(PacketKindStreamClose, id)
			response.ConnectionGeneration = packet.ConnectionGeneration
			s.endpoint.Send(response)
			return
		}
		stream := NewStream(id, packet.InBuffer, packet.OutBuffer)
		stream.generation = packet.ConnectionGeneration
		stream.remoteEpoch = packet.SourceEpoch
		stream.owner = s
		stream.opened = true
		s.streams[id] = stream
		s.resetKeepaliveTimer(stream)
		s.send(stream, allocPacket(PacketKindOpenedStream, id))
		pos := (listener.incomingPos + listener.incomingCount) % len(listener.incoming)
		listener.incoming[pos] = stream
		listener.incomingCount++
		s.updateListenerPollers(listener)
		return
	}
	stream := s.streams[id]
	if stream == nil || stream.generation != packet.ConnectionGeneration {
		return
	}
	if stream.remoteEpoch != (contracts.EpochRef{}) && stream.remoteEpoch != packet.SourceEpoch {
		return
	}
	if stream.remoteEpoch == (contracts.EpochRef{}) {
		stream.remoteEpoch = packet.SourceEpoch
	}
	switch packet.Kind {
	case PacketKindOpenedStream, PacketKindStreamAck, PacketKindStreamData, PacketKindStreamWindow:
		stream.processIncomingPacket(s, packet)
	case PacketKindStreamClose:
		_ = s.closeStream(stream, false)
	default:
		panic("unknown network packet kind")
	}
}

// send stamps immutable connection identity before transferring packet ownership.
func (s *Stack) send(stream *Stream, packet *Packet) {
	packet.ConnectionGeneration = stream.generation
	s.endpoint.Send(packet)
}

// listeners, streams

type Listener struct {
	incoming                   []*Stream
	incomingPos, incomingCount int

	closed bool

	port  uint16
	owner *Stack

	pollers pollers
}

var ErrPortAlreadyInUse = errors.New("port already in use")

func (s *Stack) OpenListener(port uint16) (*Listener, error) {
	if s.shutdown || s.quiesced || s.endpoint == nil {
		return nil, ErrListenerClosed
	}

	if _, ok := s.listeners[port]; ok {
		return nil, ErrPortAlreadyInUse
	}

	l := &Listener{
		owner:    s,
		incoming: make([]*Stream, contracts.ListenerBacklog),

		closed: false,

		port: port,
	}
	s.listeners[port] = l

	return l, nil
}

var ErrListenerClosed = errors.New("listener closed")

func (s *Stack) Accept(listener *Listener) (*Stream, error) {
	if listener.owner != s {
		panic("foreign listener")
	}
	l := listener

	if l.closed || s.quiesced {
		return nil, ErrListenerClosed
	}

	if l.incomingCount == 0 {
		return nil, syscall.EWOULDBLOCK
	}

	stream := l.incoming[l.incomingPos]
	l.incoming[l.incomingPos] = nil
	l.incomingPos++
	l.incomingCount--
	if l.incomingPos == len(l.incoming) {
		l.incomingPos = 0
	}
	s.updateListenerPollers(l)
	return stream, nil
}

func (s *Stack) updateListenerPollers(listener *Listener) {
	if s.quiesced {
		return
	}
	listener.pollers.set(s.ref, listener.closed || listener.incomingCount > 0, false, listener.closed)
}

func (s *Stack) CloseListener(listener *Listener) error {
	if listener.owner != s {
		panic("foreign listener")
	}
	l := listener

	if l.closed || s.quiesced {
		return ErrListenerClosed
	}

	return s.ReleaseListener(listener, contracts.CloseExplicit)
}

func (s *Stack) closeStream(stream *Stream, sendClose bool) error {
	if stream.closed {
		panic(stream)
	}

	if sendClose {
		s.send(stream, allocPacket(PacketKindStreamClose, stream.id))
	}

	delete(s.streams, stream.id)
	stream.closed = true
	s.updateStreamPollers(stream)

	stream.stopTimers()

	return nil
}

// A circularBuffer is a buffer for data in a TCP stream to minimize allocations
// and/or copies. The stream implementation passes buffer pointers on stream
// open, and afterwards all "data" packets are merely notifications that data
// can be read from the appropriate buffer. This way TCP reads and writes copy
// the data only twice (once from userspace to the circular buffer, and then
// from the circular buffer back to userspace), and with zero allocations.
type circularBuffer struct {
	data        []byte
	read, write int
	used        int
}

func newCircularBuffer(n int) *circularBuffer {
	return &circularBuffer{
		data: make([]byte, n),
	}
}

func (c *circularBuffer) free() int {
	return len(c.data) - c.used
}

func (c *circularBuffer) Write(data abi.ByteView) int {
	if free := c.free(); free < data.Len() {
		data = data.Slice(0, free)
	}
	n := data.Read(c.data[c.write:])
	c.write += n
	c.used += n
	if c.write == len(c.data) {
		c.write = 0
		m := data.SliceFrom(n).Read(c.data[c.write:])
		c.write += m
		c.used += m
		n += m
	}
	return n
}

func (c *circularBuffer) Read(data abi.ByteView) int {
	if c.used < data.Len() {
		data = data.Slice(0, c.used)
	}
	n := data.Write(c.data[c.read:])
	c.read += n
	c.used -= n
	if c.read == len(c.data) {
		c.read = 0
		m := data.SliceFrom(n).Write(c.data[c.read:])
		c.read += m
		c.used -= m
		n += m
	}
	return n
}

type Stream struct {
	id          ConnId
	owner       *Stack
	generation  uint64
	remoteEpoch contracts.EpochRef

	opened bool
	closed bool

	openFailedTimer *modelTimer
	ackFailedTimer  *modelTimer
	keepaliveTimer  *modelTimer

	sendPos int
	ackPos  int
	recvPos int

	// in-memory shared between both ends to skip allocating
	// Buffers belong to the connection and are shared only by its two endpoints.
	inBuffer, outBuffer *circularBuffer

	incomingAvailable int
	outgoingWindow    int

	pollers pollers
}

func (s *Stream) ID() ConnId {
	return s.id
}

// reset every time we send something that might trigger an ack
func (s *Stack) resetKeepaliveTimer(stream *Stream) {
	if stream.keepaliveTimer == nil {
		stream.keepaliveTimer = s.after(30*time.Second, func() {
			if s.shutdown || s.quiesced || stream.closed {
				return
			}

			// only send keepalive if we have not already send any data
			if stream.ackPos != stream.sendPos {
				return
			}

			packet := allocPacket(PacketKindStreamData, stream.id)
			packet.Offset = stream.sendPos
			packet.Count = 0
			s.send(stream, packet)

			s.resetKeepaliveTimer(stream)
			s.startAckFailedTimer(stream)
		})
	} else {
		stream.keepaliveTimer.Reset(15 * time.Second)
	}
}

// New writes must not postpone failure detection for earlier unacknowledged
// data. Only forward ACK progress renews an already running deadline.
func (s *Stack) startAckFailedTimer(stream *Stream) {
	if stream.ackFailedTimer == nil {
		// TODO: this time is way too low, it breaks when latency goes to 2.5s?
		stream.ackFailedTimer = s.after(5*time.Second, func() {
			if s.shutdown || s.quiesced || stream.closed {
				return
			}

			// A keepalive also requires an ACK, even when ackPos == sendPos.
			_ = s.closeStream(stream, false)
		})
	} else if stream.ackFailedTimer.event == 0 {
		stream.ackFailedTimer.Reset(5 * time.Second)
	}
}

func NewStream(id ConnId, inBuffer, outBuffer *circularBuffer) *Stream {
	return &Stream{
		id: id,

		inBuffer:       inBuffer,
		outBuffer:      outBuffer,
		outgoingWindow: len(outBuffer.data),
	}
}

func (s *Stack) OpenStream(addr netip.AddrPort) (*Stream, error) {
	if s.shutdown || s.quiesced || s.endpoint == nil {
		return nil, ErrStreamClosed
	}
	if !addr.Addr().Is4() {
		return nil, syscall.EAFNOSUPPORT
	}

	localPort := s.allocatePort()
	id := ConnId{
		Hosts: HostPair{
			SourceHost: s.endpoint.Addr(),
			DestHost:   addr.Addr(),
		},
		Ports: PortPair{
			SourcePort: localPort,
			DestPort:   addr.Port(),
		},
	}

	inBuffer := newCircularBuffer(s.endpoint.net.config.TCPBufferSize)
	outBuffer := newCircularBuffer(s.endpoint.net.config.TCPBufferSize)
	stream := NewStream(id, inBuffer, outBuffer)
	stream.owner = s
	stream.generation = s.endpoint.net.connectionGeneration()

	s.streams[stream.id] = stream
	// XXX: check stream id doesn't already exist

	packet := allocPacket(PacketKindOpenStream, id)
	packet.InBuffer = outBuffer
	packet.OutBuffer = inBuffer
	s.send(stream, packet)

	stream.openFailedTimer = s.after(s.endpoint.net.config.ConnectionTimeout, func() {
		if s.shutdown || s.quiesced {
			return
		}
		if stream.opened || stream.closed {
			return
		}
		// stream.failureReason?
		// stream.openFailed?
		_ = s.closeStream(stream, false)
	})

	return stream, nil
}

var ErrStreamClosed = errors.New("stream closed")

func (s *Stack) StreamClose(stream *Stream) error {
	if stream.owner != s {
		panic("foreign stream")
	}

	if stream.closed {
		return ErrStreamClosed
	}

	return s.closeStream(stream, true)
}

func (s *Stream) processIncomingPacket(b *Stack, packet *Packet) {
	switch packet.Kind {
	case PacketKindOpenedStream:
		if s.opened {
			return
		}
		s.opened = true
		s.openFailedTimer.Stop()
		s.openFailedTimer = nil
		b.resetKeepaliveTimer(s)
		b.updateStreamPollers(s)

	case PacketKindStreamData:
		if packet.Offset != s.recvPos {
			// XXX reject
			return
		}

		s.recvPos += packet.Count

		s.incomingAvailable += packet.Count

		response := allocPacket(PacketKindStreamAck, s.id)
		response.Offset = packet.Offset + packet.Count
		b.send(s, response)
		b.updateStreamPollers(s)

	case PacketKindStreamAck:
		if packet.Offset < s.ackPos {
			// XXX: reject
			return
		}
		progress := packet.Offset > s.ackPos
		s.ackPos = packet.Offset
		if s.ackFailedTimer != nil {
			if s.ackPos == s.sendPos {
				s.ackFailedTimer.Stop()
			} else if progress {
				s.ackFailedTimer.Reset(5 * time.Second)
			}
		}

	case PacketKindStreamWindow:
		s.outgoingWindow += packet.Count
		b.updateStreamPollers(s)
	}
}

func (s *Stack) updateStreamPollers(stream *Stream) {
	if s.quiesced {
		return
	}
	stream.pollers.set(
		s.ref, stream.closed || stream.incomingAvailable > 0,
		stream.closed || stream.outgoingWindow > 0, stream.closed,
	)
}

func (s *Stack) StreamSend(stream *Stream, data abi.ByteView) (int, error) {
	if stream.owner != s {
		panic("foreign stream")
	}
	if stream.closed || s.quiesced {
		return 0, ErrStreamClosed
	}

	if !stream.opened {
		return 0, syscall.EWOULDBLOCK
	}

	if stream.outgoingWindow == 0 {
		return 0, syscall.EWOULDBLOCK
	}

	m := min(stream.outgoingWindow, data.Len())
	written := stream.outBuffer.Write(data.Slice(0, m))
	if written != m {
		panic("bad")
	}

	offset := stream.sendPos
	stream.sendPos += written

	packet := allocPacket(PacketKindStreamData, stream.id)
	packet.Offset = offset
	packet.Count = written
	s.send(stream, packet)

	stream.outgoingWindow -= written

	s.startAckFailedTimer(stream)
	s.resetKeepaliveTimer(stream)

	s.updateStreamPollers(stream)

	return written, nil
}

func (s *Stack) StreamRecv(stream *Stream, data abi.ByteView) (int, error) {
	if stream.owner != s {
		panic("foreign stream")
	}
	if s.quiesced {
		return 0, ErrStreamClosed
	}

	if stream.incomingAvailable == 0 {
		if stream.closed {
			return 0, ErrStreamClosed
		}
		return 0, syscall.EWOULDBLOCK
	}

	m := min(stream.incomingAvailable, data.Len())
	read := stream.inBuffer.Read(data.Slice(0, m))
	if read != m {
		panic("bad")
	}
	stream.incomingAvailable -= m

	packet := allocPacket(PacketKindStreamWindow, stream.id)
	packet.Count = m
	s.send(stream, packet)

	s.updateStreamPollers(stream)

	return m, nil
}

type StreamStatus struct {
	Open   bool
	Closed bool
}

func (s *Stack) StreamStatus(stream *Stream) StreamStatus {
	return StreamStatus{
		Open:   stream.opened,
		Closed: stream.closed,
	}
}

func (s *Stack) RegisterStreamPoller(poller contracts.ReadinessSink, stream *Stream) {
	stream.pollers.add(s.ref, poller)
}

func (s *Stack) DeregisterStreamPoller(poller contracts.ReadinessSink, stream *Stream) {
	stream.pollers.remove(poller)
}

func (s *Stack) RegisterListenerPoller(poller contracts.ReadinessSink, listener *Listener) {
	listener.pollers.add(s.ref, poller)
}

func (s *Stack) DeregisterListenerPoller(poller contracts.ReadinessSink, listener *Listener) {
	listener.pollers.remove(poller)
}

func (s *Stack) ReleaseStream(stream *Stream, reason contracts.CloseReason) error {
	if stream.owner != s {
		panic("foreign stream")
	}
	if stream.closed {
		return nil
	}
	return s.closeStream(stream, reason == contracts.CloseExplicit || reason == contracts.CloseGraceful)
}

// ReleaseListener is the bounded descriptor-finalization path after Quiesce.
func (s *Stack) ReleaseListener(listener *Listener, reason contracts.CloseReason) error {
	if listener.owner != s {
		panic("foreign listener")
	}
	if listener.closed {
		return nil
	}
	if reason > contracts.CloseAbort {
		return contracts.ErrInvalidConfig
	}
	listener.closed = true
	for listener.incomingCount > 0 {
		stream := listener.incoming[listener.incomingPos]
		if !stream.closed {
			_ = s.closeStream(stream, reason == contracts.CloseExplicit || reason == contracts.CloseGraceful)
		}
		listener.incoming[listener.incomingPos] = nil
		listener.incomingPos = (listener.incomingPos + 1) % len(listener.incoming)
		listener.incomingCount--
	}
	delete(s.listeners, listener.port)
	s.updateListenerPollers(listener)
	return nil
}
