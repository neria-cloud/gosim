package net

import "net/netip"

type Direction uint8

const (
	Both Direction = iota
	Forward
)

type Link struct {
	From, To  netip.Addr
	Direction Direction
}

// LinkPolicy copies each non-nil group when submitted. Nil leaves that group's
// override unchanged; ResetLinkPolicy restores both groups to global defaults.
type LinkPolicy struct {
	Latency *LatencyConfig
	Loss    *LossConfig
}
