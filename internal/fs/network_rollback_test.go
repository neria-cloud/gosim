package fs

import (
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/kernel"
	netprovider "github.com/neria-cloud/gosim/internal/net"
)

func TestRealNetworkCollisionDoesNotPublishOrFlushFilesystem(t *testing.T) {
	must(t, withKernel(t, []contracts.Provider{netprovider.NewProvider()}, func(k *kernel.Kernel, s *Session) {
		running := create(t, k)
		must(t, k.Start(running))
		active := s.machines[running.ID].active
		inode := write(t, active.model, "/app/dirty", "unsynced")
		before := active.model.GetInodeInfo(inode)
		if before.DiskExists || before.Ops == 0 {
			t.Fatal("fixture already durable")
		}
		c, err := k.NetworkControl(contracts.CallContext{}, running.RunID)
		must(t, err)
		addr, err := c.Address(running)
		must(t, err)
		failed := create(t, k)
		must(t, c.ConfigureMachine(failed, addr, false))
		if err := k.Start(failed); err != syscall.EADDRINUSE {
			t.Fatal(err)
		}
		m := s.machines[failed.ID]
		if m == nil || m.prepared != nil || m.active != nil || m.disk != nil || m.crash != nil || m.started {
			t.Fatal("filesystem published or retained failed preparation")
		}
		if after := active.model.GetInodeInfo(inode); after != before {
			t.Fatal("failed network preparation changed filesystem durability", before, after)
		}
		must(t, c.ConfigureMachine(failed, [4]byte{}, true))
		must(t, k.Start(failed))
		if s.machines[failed.ID].active == nil {
			t.Fatal("rollback prevented retry")
		}
	}))
}
