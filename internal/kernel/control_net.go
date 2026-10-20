package kernel

import "github.com/neria-cloud/gosim/internal/contracts"

func (k *Kernel) NetworkControl(ctx contracts.CallContext, runID uint64) (contracts.NetControl, error) {
	k.Require(ctx, contracts.Net, "network control")
	c := &networkControl{kernel: k, ctx: ctx, runID: runID}
	if err := c.check(); err != nil {
		return nil, err
	}
	return c, nil
}

type networkControl struct {
	kernel *Kernel
	ctx    contracts.CallContext
	runID  uint64
}

func (c *networkControl) check() error {
	if c.runID != c.kernel.runID {
		return contracts.ErrWrongRun
	}
	if c.kernel.closed || c.ctx.Epoch.Machine.ID != 0 && !c.kernel.Accepts(c.ctx.Epoch) ||
		c.kernel.controls.NetControl == nil {
		return contracts.ErrInvalidState
	}
	return nil
}

func (c *networkControl) call(
	ref contracts.MachineRef,
	allowed []State,
	operation func(contracts.NetControl) error,
) error {
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
	return c.kernel.protected(func() error { return operation(c.kernel.controls.NetControl) })
}

func (c *networkControl) Configure(config contracts.NetConfig) error {
	return c.call(
		contracts.MachineRef{},
		nil,
		func(provider contracts.NetControl) error { return provider.Configure(config) },
	)
}

func (c *networkControl) ConfigureMachine(ref contracts.MachineRef, addr [4]byte, automatic bool) error {
	return c.call(
		ref,
		[]State{StateNew},
		func(provider contracts.NetControl) error { return provider.ConfigureMachine(ref, addr, automatic) },
	)
}

func (c *networkControl) Address(ref contracts.MachineRef) (addr [4]byte, err error) {
	err = c.call(
		ref,
		[]State{StateNew, StateRunning, StateStopped, StateCrashed},
		func(provider contracts.NetControl) error { addr, err = provider.Address(ref); return err },
	)
	return
}

func (c *networkControl) SetLinkPolicy(link contracts.NetLink, policy contracts.NetLinkPolicy) error {
	return c.call(
		contracts.MachineRef{},
		nil,
		func(provider contracts.NetControl) error { return provider.SetLinkPolicy(link, policy) },
	)
}

func (c *networkControl) ResetLinkPolicy(link contracts.NetLink) error {
	return c.call(
		contracts.MachineRef{},
		nil,
		func(provider contracts.NetControl) error { return provider.ResetLinkPolicy(link) },
	)
}

func (c *networkControl) SetConnectivity(link contracts.NetLink, connected bool) error {
	return c.call(
		contracts.MachineRef{},
		nil,
		func(provider contracts.NetControl) error { return provider.SetConnectivity(link, connected) },
	)
}
