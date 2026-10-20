//go:build sim

package net

import (
	"net/netip"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func ref(m gosim.Machine) contracts.MachineRef {
	return contracts.MachineRef{RunID: m.RunID(), ID: m.ID()}
}

func Configure(c Config) error {
	_, err := control.Call(
		kernel.NetConfigureRequest{
			Config: contracts.NetConfig{
				MinNS:               int64(c.Latency.Min),
				MaxNS:               int64(c.Latency.Max),
				Lambda:              c.Latency.Lambda,
				Fail:                c.Loss.FailProbability,
				Repair:              c.Loss.RepairProbability,
				ConnectionTimeoutNS: int64(c.ConnectionTimeout),
				TCPBufferSize:       c.TCPBufferSize,
			},
		},
		kernel.DecodeNetConfigureResult,
	)
	return err
}

func ConfigureMachine(m gosim.Machine, c MachineConfig) error {
	if err := control.Require(contracts.Net, "net.ConfigureMachine"); err != nil {
		return err
	}
	automatic := c.Addr == (netip.Addr{})
	var addr [4]byte
	if !automatic {
		if !c.Addr.Is4() {
			return gosim.ErrInvalidConfig
		}
		addr = c.Addr.As4()
	}
	_, err := control.Call(
		kernel.NetConfigureMachineRequest{Machine: ref(m), Address: addr, Automatic: automatic},
		kernel.DecodeNetConfigureMachineResult,
	)
	return err
}

func Address(m gosim.Machine) (netip.Addr, error) {
	r, err := control.Call(kernel.NetAddressRequest{Machine: ref(m)}, kernel.DecodeNetAddressResult)
	if err != nil {
		return netip.Addr{}, err
	}
	return netip.AddrFrom4(r.Address), nil
}
