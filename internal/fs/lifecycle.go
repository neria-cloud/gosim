package fs

import (
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/fs/model"
)

type Prepared struct {
	instance           *Instance
	published, aborted bool
}
type Instance struct {
	session                                *Session
	machine                                *machine
	ref                                    contracts.EpochRef
	model                                  *model.Filesystem
	fds                                    contracts.DescriptorTable
	workingDir                             int
	published, quiesced, settled, detached bool
	settleErr                              error
	mappings                               map[uintptr]*model.Mmap
	mappingOrder                           []uintptr
}

func (p *Prepared) Bindings() contracts.Bindings { return contracts.Bindings{Files: p.instance} }
func (p *Prepared) Publish() contracts.MachineInstance {
	if p.aborted || p.published || p.instance.session.closed {
		panic("invalid filesystem publication")
	}
	i := p.instance
	if i.machine.prepared != p {
		panic("filesystem preparation lost ownership")
	}
	i.machine.prepared = nil
	i.machine.active = i
	i.machine.started = true
	if i.machine.crash != nil {
		i.machine.crash.Release()
		i.machine.crash = nil
	}
	i.published, p.published = true, true
	return i
}

func (p *Prepared) Abort() {
	if p.aborted || p.published {
		return
	}
	p.aborted = true
	i := p.instance
	if i.machine.prepared == p {
		i.machine.prepared = nil
	}
	i.model.Release()
	i.model, i.fds = nil, nil
	i.detached = true
}
func (i *Instance) Quiesce() { i.quiesced = true }
func (i *Instance) Settle(kind contracts.StopKind) error {
	if i.settled {
		return i.settleErr
	}
	if !i.published || i.detached {
		return contracts.ErrInvalidState
	}
	if kind != contracts.StopGraceful && kind != contracts.StopCrash {
		return contracts.ErrInvalidConfig
	}
	i.Quiesce()
	// Record the attempt before effects, so even a provider invariant failure
	// cannot cause a second flush or consume another crash outcome.
	i.settled = true
	i.settleErr = contracts.ErrInvalidState
	if kind == contracts.StopGraceful {
		i.model.FlushEverything()
	}
	image := i.model.FreezeCrash()
	// Own the image before invoking the injected random port, which can fail.
	defer func() {
		if image != nil {
			image.Release()
		}
	}()
	var random contracts.Random
	if kind == contracts.StopCrash && i.machine.crashMode == 1 {
		random = i.session.ports.RandomFor(contracts.FS, i.ref.Machine)
	}
	disk := image.Clone(kind == contracts.StopCrash && i.machine.crashMode == 1, random)
	if i.machine.disk != nil {
		i.machine.disk.Release()
	}
	if i.machine.crash != nil {
		i.machine.crash.Release()
	}
	i.machine.disk, i.machine.crash = disk, nil
	if kind == contracts.StopCrash {
		i.machine.crash = image
		image = nil
	}
	i.settleErr = nil
	return nil
}

func (i *Instance) Detach() {
	if i.detached {
		return
	}
	i.Quiesce()
	i.detached = true
	if i.machine.active == i {
		i.machine.active = nil
	}
	if i.model != nil {
		for _, address := range i.mappingOrder {
			if mapping := i.mappings[address]; mapping != nil {
				i.model.Munmap(mapping)
				delete(i.mappings, address)
			}
		}
		i.mappings, i.mappingOrder = nil, nil
		i.model.Release()
		i.model = nil
	}
	i.fds = nil
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
