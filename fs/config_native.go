//go:build !sim

package fs

import "github.com/neria-cloud/gosim"

func ConfigureMachine(gosim.Machine, Config) error { return gosim.ErrNotSim }
