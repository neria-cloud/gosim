package kernel

import "github.com/neria-cloud/gosim/internal/contracts"

// FilesystemControl is the supervisor-side control boundary. Generated control
// routes pass the public handle's run ID separately because recovery IDs are
// session-local. No concrete provider or model crosses this boundary.
func (k *Kernel) FilesystemControl(ctx contracts.CallContext, runID uint64) (contracts.FSControl, error) {
	k.Require(ctx, contracts.FS, "filesystem control")
	c := &filesystemControl{kernel: k, ctx: ctx, runID: runID}
	if err := c.check(); err != nil {
		return nil, err
	}
	return c, nil
}

type filesystemControl struct {
	kernel *Kernel
	ctx    contracts.CallContext
	runID  uint64
}

func (c *filesystemControl) check() error {
	if c.runID != c.kernel.runID {
		return contracts.ErrWrongRun
	}
	if c.kernel.closed || c.ctx.Epoch.Machine.ID != 0 && !c.kernel.Accepts(c.ctx.Epoch) {
		return contracts.ErrInvalidState
	}
	if c.kernel.controls.FSControl == nil {
		return contracts.ErrInvalidState
	}
	return nil
}

func (c *filesystemControl) call(ref contracts.MachineRef, allowed []State, operation func(contracts.FSControl) error) error {
	if err := c.check(); err != nil {
		return err
	}
	if allowed != nil {
		m, err := c.kernel.Lookup(ref)
		if err != nil {
			return err
		}
		valid := false
		for _, state := range allowed {
			if m.state == state {
				valid = true
			}
		}
		if !valid {
			return contracts.ErrInvalidState
		}
	}
	return c.kernel.protected(func() error { return operation(c.kernel.controls.FSControl) })
}

func (c *filesystemControl) ConfigureMachine(ref contracts.MachineRef, config contracts.FSConfig) error {
	return c.call(ref, []State{StateNew}, func(provider contracts.FSControl) error { return provider.ConfigureMachine(ref, config) })
}

func (c *filesystemControl) SetCrashMode(ref contracts.MachineRef, mode uint8) error {
	return c.call(ref, []State{StateNew, StateRunning, StateStopped, StateCrashed}, func(provider contracts.FSControl) error { return provider.SetCrashMode(ref, mode) })
}

func (c *filesystemControl) SetSyncFault(ref contracts.MachineRef, fault contracts.FSFaultConfig) error {
	return c.call(ref, []State{StateNew, StateRunning, StateStopped, StateCrashed}, func(provider contracts.FSControl) error { return provider.SetSyncFault(ref, fault) })
}

func (c *filesystemControl) SetReadOnly(ref contracts.MachineRef, enabled bool) error {
	return c.call(ref, []State{StateNew, StateRunning, StateStopped, StateCrashed}, func(provider contracts.FSControl) error { return provider.SetReadOnly(ref, enabled) })
}

func (c *filesystemControl) Stats(ref contracts.MachineRef) (stats contracts.FSStorageStats, err error) {
	err = c.call(ref, []State{StateNew, StateRunning, StateStopped, StateCrashed}, func(provider contracts.FSControl) error { stats, err = provider.Stats(ref); return err })
	return
}

func (c *filesystemControl) InspectInode(ref contracts.MachineRef, inode int) (info contracts.InodeInfo, err error) {
	err = c.call(ref, []State{StateNew, StateRunning, StateStopped, StateCrashed}, func(provider contracts.FSControl) error { info, err = provider.InspectInode(ref, inode); return err })
	return
}

func (c *filesystemControl) Enumerate(ref contracts.MachineRef, maxStates uint64) (id uint64, err error) {
	err = c.call(ref, []State{StateCrashed}, func(provider contracts.FSControl) error { id, err = provider.Enumerate(ref, maxStates); return err })
	return
}

func (c *filesystemControl) NextSnapshot(iteratorID uint64) (id uint64, ok bool, err error) {
	err = c.call(contracts.MachineRef{}, nil, func(provider contracts.FSControl) error { id, ok, err = provider.NextSnapshot(iteratorID); return err })
	return
}

func (c *filesystemControl) CloseIterator(id uint64) error {
	return c.call(contracts.MachineRef{}, nil, func(provider contracts.FSControl) error { return provider.CloseIterator(id) })
}

func (c *filesystemControl) Restore(ref contracts.MachineRef, id uint64) error {
	return c.call(ref, []State{StateNew, StateStopped, StateCrashed}, func(provider contracts.FSControl) error { return provider.Restore(ref, id) })
}

func (c *filesystemControl) CloseSnapshot(id uint64) error {
	return c.call(contracts.MachineRef{}, nil, func(provider contracts.FSControl) error { return provider.CloseSnapshot(id) })
}
