//go:build sim

package net

import (
	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func linkValue(link Link) (contracts.NetLink, error) {
	if !link.From.Is4() || !link.To.Is4() || link.Direction > Forward {
		return contracts.NetLink{}, gosim.ErrInvalidConfig
	}
	return contracts.NetLink{From: link.From.As4(), To: link.To.As4(), Direction: uint8(link.Direction)}, nil
}

func SetLinkPolicy(link Link, policy LinkPolicy) error {
	if err := control.Require(contracts.Net, "net.SetLinkPolicy"); err != nil {
		return err
	}
	l, err := linkValue(link)
	if err != nil {
		return err
	}
	var p contracts.NetLinkPolicy
	if policy.Latency != nil {
		c := *policy.Latency
		p.SetLatency = true
		p.MinNS = int64(c.Min)
		p.MaxNS = int64(c.Max)
		p.Lambda = c.Lambda
	}
	if policy.Loss != nil {
		c := *policy.Loss
		p.SetLoss = true
		p.Fail = c.FailProbability
		p.Repair = c.RepairProbability
	}
	_, err = control.Call(kernel.NetSetLinkPolicyRequest{Link: l, Policy: p}, kernel.DecodeNetSetLinkPolicyResult)
	return err
}

func ResetLinkPolicy(link Link) error {
	if err := control.Require(contracts.Net, "net.ResetLinkPolicy"); err != nil {
		return err
	}
	l, err := linkValue(link)
	if err != nil {
		return err
	}
	_, err = control.Call(kernel.NetResetLinkPolicyRequest{Link: l}, kernel.DecodeNetResetLinkPolicyResult)
	return err
}

func Partition(link Link) error {
	if err := control.Require(contracts.Net, "net.Partition"); err != nil {
		return err
	}
	l, err := linkValue(link)
	if err != nil {
		return err
	}
	_, err = control.Call(kernel.NetSetConnectivityRequest{Link: l, Connected: false}, kernel.DecodeNetSetConnectivityResult)
	return err
}

func Repair(link Link) error {
	if err := control.Require(contracts.Net, "net.Repair"); err != nil {
		return err
	}
	l, err := linkValue(link)
	if err != nil {
		return err
	}
	_, err = control.Call(kernel.NetSetConnectivityRequest{Link: l, Connected: true}, kernel.DecodeNetSetConnectivityResult)
	return err
}
