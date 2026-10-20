package kernel

import (
	"errors"
	"math"
	"slices"

	"github.com/neria-cloud/gosim/internal/contracts"
)

var (
	ErrPollClosing error = contracts.Sentinel("poll descriptor is closing")
	ErrPollTimeout error = contracts.Sentinel("poll deadline exceeded")
	ErrNotPollable error = contracts.Sentinel("descriptor is not pollable")
	errPending     error = contracts.Sentinel("invocation awaits completion")
)

type pollDirection struct {
	timedOut   bool
	generation uint64
	event      contracts.EventID
	waiters    map[uint64]struct{}
}
type pollRegistration struct {
	id          uint64
	epoch       contracts.EpochRef
	fd          int
	resource    contracts.Pollable
	sink        *registrationSink
	ready       contracts.Readiness
	unblocked   bool
	read, write pollDirection
}
type registrationSink struct {
	kernel *Kernel
	id     uint64
}

func (s *registrationSink) Notify(epoch contracts.EpochRef, fd int, ready contracts.Readiness) {
	p := s.kernel.polls[s.id]
	if p == nil || p.epoch != epoch || p.fd != fd || !s.kernel.Accepts(epoch) {
		return
	}
	s.kernel.notifyPoll(p, ready)
}

func (k *Kernel) PollOpen(ctx contracts.CallContext, fd int) (uint64, error) {
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return 0, err
	}
	resource, err := run.descriptors.Lookup(fd)
	if err != nil {
		return 0, err
	}
	k.Require(ctx, resource.Owner(), "poll-open")
	pollable, ok := resource.(contracts.Pollable)
	if !ok {
		return 0, ErrNotPollable
	}
	if k.nextPollID == 0 {
		k.invariant("poll registration ID space exhausted")
		return 0, contracts.ErrInvalidState
	}
	id := k.nextPollID
	k.nextPollID++
	p := &pollRegistration{id: id, epoch: ctx.Epoch, fd: fd, resource: pollable, sink: &registrationSink{kernel: k, id: id}}
	p.read.waiters, p.write.waiters = make(map[uint64]struct{}), make(map[uint64]struct{})
	k.polls[id] = p
	if err := pollable.Subscribe(ctx, p.sink); err != nil {
		delete(k.polls, id)
		if cleanupErr := k.protected(func() error { pollable.Unsubscribe(ctx, p.sink); return nil }); cleanupErr != nil {
			k.cleanupFailure("poll subscription rollback", errors.Join(err, cleanupErr))
			return 0, errors.Join(err, cleanupErr)
		}
		return 0, err
	}
	return id, nil
}

func (k *Kernel) poll(ctx contracts.CallContext, id uint64) (*pollRegistration, error) {
	if !k.Accepts(ctx.Epoch) {
		return nil, contracts.ErrInvalidState
	}
	p := k.polls[id]
	if p == nil || p.epoch != ctx.Epoch {
		return nil, ErrPollClosing
	}
	return p, nil
}

func direction(p *pollRegistration, mode uint8) (*pollDirection, contracts.Readiness, error) {
	switch mode {
	case 'r':
		return &p.read, contracts.ReadReady, nil
	case 'w':
		return &p.write, contracts.WriteReady, nil
	}
	return nil, 0, contracts.ErrInvalidConfig
}

func pollState(p *pollRegistration, d *pollDirection) error {
	if p.unblocked {
		return ErrPollClosing
	}
	if d.timedOut {
		return ErrPollTimeout
	}
	return nil
}

func (k *Kernel) PollWait(ctx contracts.CallContext, id uint64, mode uint8) error {
	p, err := k.poll(ctx, id)
	if err != nil {
		return err
	}
	d, ready, err := direction(p, mode)
	if err != nil {
		return err
	}
	if err := pollState(p, d); err != nil {
		return err
	}
	if p.ready&(ready|contracts.Hangup|contracts.ErrorReady) != 0 {
		return nil
	}
	if !k.dispatcher.ownsInvocation(ctx) {
		return contracts.ErrInvalidState
	}
	d.waiters[ctx.InvocationID] = struct{}{}
	return errPending
}

func (k *Kernel) PollReset(ctx contracts.CallContext, id uint64, mode uint8) error {
	p, err := k.poll(ctx, id)
	if err != nil {
		return err
	}
	d, ready, err := direction(p, mode)
	if err != nil {
		return err
	}
	if err := pollState(p, d); err != nil {
		return err
	}
	p.ready &^= ready
	return nil
}

func (k *Kernel) PollDeadline(ctx contracts.CallContext, id uint64, duration int64, mode uint8) error {
	p, err := k.poll(ctx, id)
	if err != nil {
		return err
	}
	if mode != 'r' && mode != 'w' && mode != 'r'+'w' {
		return contracts.ErrInvalidConfig
	}
	now := k.Ports().Clock.NowNanos()
	if duration > 0 && duration > math.MaxInt64-now {
		return contracts.ErrInvalidConfig
	}
	directions := []*pollDirection{}
	if mode == 'r' || mode == 'r'+'w' {
		directions = append(directions, &p.read)
	}
	if mode == 'w' || mode == 'r'+'w' {
		directions = append(directions, &p.write)
	}
	for _, d := range directions {
		if d.generation == math.MaxUint64 {
			k.invariant("poll deadline generation exhausted")
			return contracts.ErrInvalidState
		}
	}
	for _, d := range directions {
		d.generation++
		generation := d.generation
		if d.event != 0 {
			k.Ports().Events.Cancel(d.event)
			d.event = 0
		}
		d.timedOut = duration < 0
		if duration > 0 {
			directionMode := uint8('w')
			if d == &p.read {
				directionMode = 'r'
			}
			event, err := k.Ports().Events.Schedule(p.epoch, now+duration, func() {
				k.expirePoll(id, generation, directionMode)
			})
			if err != nil {
				k.invariant("cannot schedule validated poll deadline: " + err.Error())
				return err
			}
			d.event = event
		}
		if d.timedOut {
			k.completePollWaiters(p, d, ErrPollTimeout)
		}
	}
	return nil
}

func (k *Kernel) expirePoll(id, generation uint64, mode uint8) {
	p := k.polls[id]
	if p == nil || !k.Accepts(p.epoch) {
		return
	}
	d, _, err := direction(p, mode)
	if err != nil || d.generation != generation {
		return
	}
	d.event, d.timedOut = 0, true
	k.completePollWaiters(p, d, ErrPollTimeout)
}

func (k *Kernel) PollUnblock(ctx contracts.CallContext, id uint64) error {
	p, err := k.poll(ctx, id)
	if err != nil {
		return err
	}
	p.unblocked = true
	k.completePollWaiters(p, &p.read, ErrPollClosing)
	k.completePollWaiters(p, &p.write, ErrPollClosing)
	return nil
}

func (k *Kernel) PollClose(ctx contracts.CallContext, id uint64) error {
	p, err := k.poll(ctx, id)
	if err != nil {
		return err
	}
	return k.closePoll(p)
}

func (k *Kernel) closePoll(p *pollRegistration) error {
	delete(k.polls, p.id) // stale sinks cannot target a later registration
	p.unblocked = true
	for _, d := range []*pollDirection{&p.read, &p.write} {
		if d.event != 0 {
			k.Ports().Events.Cancel(d.event)
			d.event = 0
		}
		k.completePollWaiters(p, d, ErrPollClosing)
	}
	return k.protected(func() error { p.resource.Unsubscribe(contracts.CallContext{Epoch: p.epoch}, p.sink); return nil })
}

func (k *Kernel) closePolls(epoch contracts.EpochRef, fd int) error {
	ids := make([]uint64, 0, len(k.polls))
	for id, p := range k.polls {
		if p.epoch == epoch && (fd < 0 || p.fd == fd) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	failures := make([]error, 0, len(ids))
	for _, id := range ids {
		failures = append(failures, k.closePoll(k.polls[id]))
	}
	return errors.Join(failures...)
}

func (k *Kernel) Notify(epoch contracts.EpochRef, fd int, ready contracts.Readiness) {
	if !k.Accepts(epoch) {
		return
	}
	ids := make([]uint64, 0, len(k.polls))
	for id, p := range k.polls {
		if p.epoch == epoch && p.fd == fd {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		k.notifyPoll(k.polls[id], ready)
	}
}

func (k *Kernel) notifyPoll(p *pollRegistration, ready contracts.Readiness) {
	if ready & ^(contracts.ReadReady|contracts.WriteReady|contracts.Hangup|contracts.ErrorReady) != 0 {
		k.invariant("invalid readiness bits")
		return
	}
	p.ready = ready
	if ready&(contracts.ReadReady|contracts.Hangup|contracts.ErrorReady) != 0 {
		k.completePollWaiters(p, &p.read, pollState(p, &p.read))
	}
	if ready&(contracts.WriteReady|contracts.Hangup|contracts.ErrorReady) != 0 {
		k.completePollWaiters(p, &p.write, pollState(p, &p.write))
	}
}

func (k *Kernel) completePollWaiters(p *pollRegistration, d *pollDirection, err error) {
	ids := make([]uint64, 0, len(d.waiters))
	for id := range d.waiters {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		delete(d.waiters, id)
		if k.Accepts(p.epoch) {
			code, resultErr := pollCode(err)
			result, _, _ := controlRouteResult(PollWaitResult{Code: code}, resultErr)
			_ = k.dispatcher.Complete(id, result)
		}
	}
}

func pollCode(err error) (int, error) {
	switch err {
	case nil:
		return 0, nil
	case ErrPollClosing:
		return 1, nil
	case ErrPollTimeout:
		return 2, nil
	case ErrNotPollable:
		return 3, nil
	}
	return 0, err
}
