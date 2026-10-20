//go:build sim

package fs

import (
	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func ref(m gosim.Machine) contracts.MachineRef {
	return contracts.MachineRef{RunID: m.RunID(), ID: m.ID()}
}

func ConfigureMachine(m gosim.Machine, c Config) error {
	config := contracts.FSConfig{
		WorkingDir:    c.WorkingDir,
		CapacityBytes: c.CapacityBytes,
		UID:           c.UID,
		GID:           c.GID,
		LatencyNS:     int64(c.Latency),
	}
	_, err := control.Call(
		kernel.FSConfigureMachineRequest{Machine: ref(m), Config: config},
		kernel.DecodeFSConfigureMachineResult,
	)
	return err
}
