//go:build !sim

package net

import (
	"net/netip"

	"github.com/neria-cloud/gosim"
)

func Configure(Config) error                              { return gosim.ErrNotSim }
func ConfigureMachine(gosim.Machine, MachineConfig) error { return gosim.ErrNotSim }
func Address(gosim.Machine) (netip.Addr, error)           { return netip.Addr{}, gosim.ErrNotSim }
