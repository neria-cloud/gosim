package net

import (
	"slices"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/net/model"
)

type Prepared struct {
	instance           *Instance
	reservation        model.Reservation
	published, aborted bool
}
type Instance struct {
	session                                *Session
	machine                                *machine
	ref                                    contracts.EpochRef
	fds                                    contracts.DescriptorTable
	stack                                  *model.Stack
	sockets                                map[int]*Socket
	published, quiesced, settled, detached bool
}

func (p *Prepared) Bindings() contracts.Bindings { return contracts.Bindings{Sockets: p.instance} }
func (p *Prepared) Publish() contracts.MachineInstance {
	i := p.instance
	if p.aborted || p.published || i.session.closed || i.machine.prepared != p {
		panic("invalid network publication")
	}
	i.session.network.Attach(p.reservation, i.ref, i.stack)
	i.machine.address = p.reservation.Address().As4()
	i.machine.started = true
	i.machine.active = i
	i.machine.prepared = nil
	i.published = true
	p.published = true
	return i
}

func (p *Prepared) Abort() {
	if p.aborted || p.published {
		return
	}
	p.aborted = true
	i := p.instance
	i.session.network.AbortReservation(p.reservation)
	if i.machine.prepared == p {
		i.machine.prepared = nil
	}
	i.stack.Shutdown(contracts.StopCrash)
	i.stack = nil
	i.fds = nil
	i.detached = true
}

func (i *Instance) Quiesce() {
	if i.quiesced {
		return
	}
	i.quiesced = true
	if i.stack != nil {
		i.stack.Quiesce()
	}
	for _, fd := range i.socketIDs() {
		i.sockets[fd].retireSubscriptions()
	}
}

func (i *Instance) Settle(kind contracts.StopKind) error {
	if i.settled {
		return nil
	}
	if !i.published || i.detached {
		return contracts.ErrInvalidState
	}
	if kind > contracts.StopCrash {
		return contracts.ErrInvalidConfig
	}
	i.Quiesce()
	i.settled = true
	return nil
}

func (i *Instance) Detach() {
	if i.detached {
		return
	}
	i.Quiesce()
	// Kernel normally finalized these descriptors already. Defensive disposal
	// retires remaining provider resources without taking descriptor-table ownership.
	for _, fd := range i.socketIDs() {
		if err := i.sockets[fd].Release(contracts.CloseCrash); err != nil {
			panic(err)
		}
	}
	if i.stack != nil {
		i.stack.Shutdown(contracts.StopCrash)
		i.session.network.Detach(i.ref)
		i.stack = nil
	}
	if i.machine.active == i {
		i.machine.active = nil
	}
	i.fds = nil
	i.sockets = nil
	i.detached = true
}

func (i *Instance) socketIDs() []int {
	ids := make([]int, 0, len(i.sockets))
	for fd := range i.sockets {
		ids = append(ids, fd)
	}
	slices.Sort(ids)
	return ids
}

func (i *Instance) validateCall(ctx contracts.CallContext) error {
	if ctx.Epoch.Machine.RunID != i.ref.Machine.RunID {
		return contracts.ErrWrongRun
	}
	if ctx.Epoch != i.ref || !i.published || i.quiesced || i.detached || i.machine.active != i {
		return contracts.ErrInvalidState
	}
	return nil
}
