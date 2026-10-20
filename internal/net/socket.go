package net

import (
	"math"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/net/model"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type Socket struct {
	instance      *Instance
	fd            int
	bound         bool
	boundAddress  contracts.Address
	stream        *model.Stream
	listener      *model.Listener
	released      bool
	closeReason   contracts.CloseReason
	generation    uint64
	subscriptions []*socketReadiness
}
type socketReadiness struct {
	socket     *Socket
	sink       contracts.ReadinessSink
	generation uint64
	active     bool
}

func (r *socketReadiness) Notify(ref contracts.EpochRef, _ int, mask contracts.Readiness) {
	s := r.socket
	if !r.active || s.released || s.instance.quiesced || s.instance.detached || s.generation != r.generation ||
		ref != s.instance.ref {
		return
	}
	r.sink.Notify(ref, s.fd, mask)
}
func (*Socket) Owner() contracts.Feature { return contracts.Net }
func (s *Socket) Release(reason contracts.CloseReason) error {
	if s.released {
		return nil
	}
	if reason > contracts.CloseAbort {
		return contracts.ErrInvalidConfig
	}
	s.released = true
	s.closeReason = reason
	s.retireSubscriptions()
	var err error
	if s.stream != nil {
		err = s.instance.stack.ReleaseStream(s.stream, reason)
		s.stream = nil
	}
	if s.listener != nil {
		err = s.instance.stack.ReleaseListener(s.listener, reason)
		s.listener = nil
	}
	delete(s.instance.sockets, s.fd)
	return err
}

func (s *Socket) check(ctx contracts.CallContext) error {
	if err := s.instance.validateCall(ctx); err != nil {
		return err
	}
	if s.released {
		return syscall.EBADF
	}
	return nil
}

func (s *Socket) Read(ctx contracts.CallContext, buffer abi.ByteView) (int, error) {
	if err := s.check(ctx); err != nil {
		return 0, err
	}
	if s.stream == nil {
		return 0, syscall.ENOTCONN
	}
	if buffer.Len() == 0 {
		return 0, nil
	}
	n, err := s.instance.stack.StreamRecv(s.stream, buffer)
	if err == model.ErrStreamClosed {
		return 0, syscall.EPIPE
	}
	return n, err
}

func (s *Socket) Write(ctx contracts.CallContext, buffer abi.ByteView) (int, error) {
	if err := s.check(ctx); err != nil {
		return 0, err
	}
	if s.stream == nil {
		return 0, syscall.ENOTCONN
	}
	if buffer.Len() == 0 {
		return 0, nil
	}
	n, err := s.instance.stack.StreamSend(s.stream, buffer)
	if err == model.ErrStreamClosed {
		return 0, syscall.EPIPE
	}
	return n, err
}

func (s *Socket) attach(r *socketReadiness) {
	if s.stream != nil {
		s.instance.stack.RegisterStreamPoller(r, s.stream)
	} else if s.listener != nil {
		s.instance.stack.RegisterListenerPoller(r, s.listener)
	} else {
		r.Notify(s.instance.ref, 0, 0)
	}
}

func (s *Socket) detach(r *socketReadiness) {
	r.active = false
	if s.stream != nil {
		s.instance.stack.DeregisterStreamPoller(r, s.stream)
	}
	if s.listener != nil {
		s.instance.stack.DeregisterListenerPoller(r, s.listener)
	}
	r.sink = nil
}

func (s *Socket) Subscribe(ctx contracts.CallContext, sink contracts.ReadinessSink) error {
	if err := s.check(ctx); err != nil {
		return err
	}
	if sink == nil {
		return contracts.ErrInvalidConfig
	}
	for _, r := range s.subscriptions {
		if r.sink == sink {
			return contracts.ErrInvalidState
		}
	}
	r := &socketReadiness{socket: s, sink: sink, generation: s.generation, active: true}
	s.subscriptions = append(s.subscriptions, r)
	s.attach(r)
	return nil
}

func (s *Socket) Unsubscribe(ctx contracts.CallContext, sink contracts.ReadinessSink) {
	if ctx.Epoch != s.instance.ref {
		return
	}
	for i, r := range s.subscriptions {
		if r.sink == sink {
			s.detach(r)
			copy(s.subscriptions[i:], s.subscriptions[i+1:])
			s.subscriptions[len(s.subscriptions)-1] = nil
			s.subscriptions = s.subscriptions[:len(s.subscriptions)-1]
			return
		}
	}
}

func (s *Socket) retireSubscriptions() {
	for _, r := range s.subscriptions {
		s.detach(r)
	}
	s.subscriptions = nil
}

// A blank socket can be polled before connect/listen. Replace its readiness
// bridges so a retained callback from the prior binding cannot affect new state.
func (s *Socket) rebind() {
	if s.generation == math.MaxUint64 {
		panic("socket readiness generation exhausted")
	}
	s.generation++
	for index, old := range s.subscriptions {
		sink := old.sink
		s.detach(old)
		r := &socketReadiness{socket: s, sink: sink, generation: s.generation, active: true}
		s.subscriptions[index] = r
		s.attach(r)
	}
}
