package net

import (
	"net/netip"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/net/model"
)

func (s *Session) Configure(config contracts.NetConfig) error {
	if s.closed {
		return contracts.ErrInvalidState
	}
	if err := contracts.ValidateNetConfig(config); err != nil {
		return err
	}
	return s.network.ApplyControl(model.ControlRequest{Kind: model.ReplaceGlobal, Config: model.Config{Latency: model.LatencyConfig{Min: time.Duration(config.MinNS), Max: time.Duration(config.MaxNS), Lambda: config.Lambda}, Loss: model.LossConfig{FailProbability: config.Fail, RepairProbability: config.Repair}}})
}

func (s *Session) ConfigureMachine(ref contracts.MachineRef, addr [4]byte, automatic bool) error {
	if err := s.checkRef(ref); err != nil {
		return err
	}
	if m := s.machines[ref.ID]; m != nil && (m.started || m.prepared != nil || m.active != nil) {
		return contracts.ErrInvalidState
	}
	ip := netip.AddrFrom4(addr)
	if !automatic && (ip.IsUnspecified() || ip.IsMulticast()) || automatic && addr != ([4]byte{}) {
		return contracts.ErrInvalidConfig
	}
	m := s.machine(ref)
	m.configured = addr
	m.automatic = automatic
	return nil
}

func (s *Session) Address(ref contracts.MachineRef) ([4]byte, error) {
	if err := s.checkRef(ref); err != nil {
		return [4]byte{}, err
	}
	m := s.machines[ref.ID]
	if m == nil || !m.started {
		return [4]byte{}, contracts.ErrNotStarted
	}
	return m.address, nil
}

func linkPair(link contracts.NetLink) model.HostPair {
	return model.HostPair{SourceHost: netip.AddrFrom4(link.From), DestHost: netip.AddrFrom4(link.To)}
}

func (s *Session) SetLinkPolicy(link contracts.NetLink, policy contracts.NetLinkPolicy) error {
	if s.closed {
		return contracts.ErrInvalidState
	}
	if err := contracts.ValidateNetLink(link); err != nil {
		return err
	}
	if err := contracts.ValidateNetLinkPolicy(policy); err != nil {
		return err
	}
	var latency *model.LatencyConfig
	var loss *model.LossConfig
	if policy.SetLatency {
		latency = &model.LatencyConfig{Min: time.Duration(policy.MinNS), Max: time.Duration(policy.MaxNS), Lambda: policy.Lambda}
	}
	if policy.SetLoss {
		loss = &model.LossConfig{FailProbability: policy.Fail, RepairProbability: policy.Repair}
	}
	return s.network.SetLinkPolicy(linkPair(link), model.ConnectivityDirection(link.Direction), latency, loss)
}

func (s *Session) ResetLinkPolicy(link contracts.NetLink) error {
	if s.closed {
		return contracts.ErrInvalidState
	}
	if err := contracts.ValidateNetLink(link); err != nil {
		return err
	}
	return s.network.ResetLinkPolicy(linkPair(link), model.ConnectivityDirection(link.Direction))
}

func (s *Session) SetConnectivity(link contracts.NetLink, connected bool) error {
	if s.closed {
		return contracts.ErrInvalidState
	}
	if err := contracts.ValidateNetLink(link); err != nil {
		return err
	}
	return s.network.SetConnectivity(linkPair(link), connected, model.ConnectivityDirection(link.Direction))
}
