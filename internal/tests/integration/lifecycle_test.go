//go:build sim

package behavior_test

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	simnet "github.com/neria-cloud/gosim/net"
)

// A network preparation error occurs after filesystem preparation. Retrying
// must preserve the restored disk image and leave the existing address owner live.
func TestSecondProviderRollbackPreservesDisk(t *testing.T) {
	address := netip.MustParseAddr("10.40.0.1")
	ready := make(chan struct{})
	owner := testsupport.StartConfigured(t, gosim.MachineConfig{Main: func() {
		l, err := net.Listen("tcp", "10.40.0.1:9000")
		testsupport.Check(t, err)
		defer l.Close()
		close(ready)
		select {}
	}}, address)
	<-ready
	source := testsupport.Start(t, func() {
		file, err := os.OpenFile("durable", os.O_CREATE|os.O_WRONLY, 0o600)
		testsupport.Check(t, err)
		_, err = file.Write([]byte("before"))
		testsupport.Check(t, err)
		testsupport.Check(t, file.Sync())
		testsupport.Check(t, file.Close())
		dir, err := os.Open(".")
		testsupport.Check(t, err)
		testsupport.Check(t, dir.Sync())
		testsupport.Check(t, dir.Close())
		gosim.CrashSelf()
	})
	testsupport.Wait(t, source)
	iterator, err := simfs.CrashStates(source, simfs.EnumerateOptions{MaxStates: 1})
	testsupport.Check(t, err)
	snapshot, ok, err := iterator.Next()
	testsupport.Check(t, err)
	if !ok {
		t.Fatal("missing durable snapshot")
	}
	testsupport.Check(t, iterator.Close())
	testsupport.Check(t, source.Destroy())
	child, err := gosim.NewMachine(gosim.MachineConfig{Main: func() { t.Fatal("failed prepare ran application") }})
	testsupport.Check(t, err)
	testsupport.Check(t, simfs.Restore(child, snapshot))
	testsupport.Check(t, snapshot.Close())
	testsupport.Check(t, simnet.ConfigureMachine(child, simnet.MachineConfig{Addr: address}))
	if err := child.Start(); err == nil {
		t.Fatal("address collision accepted")
	}
	state, err := child.State()
	testsupport.Check(t, err)
	if state != gosim.New {
		t.Fatalf("failed start lost previous state: %v", state)
	}
	testsupport.Check(t, simnet.ConfigureMachine(child, simnet.MachineConfig{Addr: netip.MustParseAddr("10.40.0.2")}))
	testsupport.Check(t, child.SetMain(func() {
		data, err := os.ReadFile("durable")
		testsupport.Check(t, err)
		if string(data) != "before" {
			t.Fatalf("rollback changed disk: %q", data)
		}
		conn, err := net.Dial("tcp", "10.40.0.1:9000")
		testsupport.Check(t, err)
		testsupport.Check(t, conn.Close())
		testsupport.Check(t, simfs.SetCrashMode(gosim.CurrentMachine(), simfs.DurableOnly))
	}))
	testsupport.Check(t, child.Start())
	testsupport.Wait(t, child)
	testsupport.Check(t, owner.Crash())
	testsupport.Check(t, child.Destroy())
	if _, err := child.State(); !errors.Is(err, gosim.ErrMachineNotFound) {
		t.Fatal("destroyed handle accepted", err)
	}
}
