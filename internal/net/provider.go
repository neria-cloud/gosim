// Package net implements translated IPv4 TCP services over an owned model.
package net

import (
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/net/model"
)

type Provider struct{}

func NewProvider() contracts.Provider        { return &Provider{} }
func (*Provider) Feature() contracts.Feature { return contracts.Net }
func (*Provider) Open(ports contracts.Ports) (contracts.ProviderSession, error) {
	if err := contracts.ValidatePorts(ports); err != nil {
		return nil, err
	}
	random := ports.RandomFor(contracts.Net, contracts.MachineRef{})
	if random == nil {
		return nil, contracts.ErrInvalidConfig
	}
	return &Session{ports: ports, network: model.NewNetwork(ports.Clock, ports.Events, random, ports.Trace), machines: make(map[uint64]*machine), destroyed: make(map[uint64]bool)}, nil
}
