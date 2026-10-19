//go:build !sim

package net

import "github.com/neria-cloud/gosim"

func SetLinkPolicy(Link, LinkPolicy) error { return gosim.ErrNotSim }
func ResetLinkPolicy(Link) error           { return gosim.ErrNotSim }
func Partition(Link) error                 { return gosim.ErrNotSim }
func Repair(Link) error                    { return gosim.ErrNotSim }
