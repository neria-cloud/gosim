//go:build sim

// Package testsupport contains checked setup helpers for simulation fixtures.
package testsupport

import (
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	simnet "github.com/neria-cloud/gosim/net"
	"github.com/neria-cloud/gosim/rt"
)

func Check(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func Start(t testing.TB, main func()) gosim.Machine {
	t.Helper()
	return StartConfigured(t, gosim.MachineConfig{Main: main})
}

func StartConfigured(t testing.TB, config gosim.MachineConfig, addresses ...netip.Addr) gosim.Machine {
	t.Helper()
	m, err := gosim.NewMachine(config)
	Check(t, err)
	if len(addresses) > 1 {
		t.Fatal("fixture accepts at most one machine address")
	}
	if len(addresses) == 1 {
		Check(t, simnet.ConfigureMachine(m, simnet.MachineConfig{Addr: addresses[0]}))
	}
	Check(t, m.Start())
	return m
}

func Wait(t testing.TB, m gosim.Machine) gosim.Exit {
	t.Helper()
	result, err := m.Wait()
	Check(t, err)
	return result
}

func Label(t testing.TB, m gosim.Machine) string {
	t.Helper()
	label, err := m.Label()
	Check(t, err)
	return label
}

func Inode(t testing.TB, m gosim.Machine, inode int) simfs.InodeInfo {
	t.Helper()
	info, err := simfs.InspectInode(m, inode)
	Check(t, err)
	return info
}
func Timeout(t testing.TB, timeout time.Duration) { t.Helper(); Check(t, rt.SetTimeout(timeout)) }
