//go:build sim

package behavior_test

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func TestPublicStorageCapacityAndReadOnly(t *testing.T) {
	ready := make(chan struct{})
	resume := make(chan struct{})
	blocked := make(chan error, 1)
	m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
		f, err := os.OpenFile("bounded", os.O_CREATE|os.O_RDWR, 0o600)
		testsupport.Check(t, err)
		defer f.Close()
		_, err = f.WriteAt([]byte("x"), 0)
		testsupport.Check(t, err)
		close(ready)
		<-resume
		_, err = f.WriteAt([]byte("y"), 0)
		blocked <- err
		<-resume
		_, err = f.WriteAt([]byte("z"), 0)
		testsupport.Check(t, err)
		if n, err := f.WriteAt([]byte("q"), 1024); n != 0 || !errors.Is(err, syscall.ENOSPC) {
			t.Fatalf("full-volume write: n=%d err=%v", n, err)
		}
		info, err := f.Stat()
		testsupport.Check(t, err)
		if info.Size() != 1 {
			t.Fatalf("full-volume write changed length: %d", info.Size())
		}
	}})
	testsupport.Check(t, err)
	config := simfs.DefaultConfig()
	config.CapacityBytes = 2 * 1024
	testsupport.Check(t, simfs.ConfigureMachine(m, config))
	stats, err := simfs.Stats(m)
	testsupport.Check(t, err)
	if stats.TotalBytes != 2048 || stats.UsedBytes != 1024 || stats.FreeBytes != 1024 || stats.BlockSize != 1024 {
		t.Fatalf("prestart stats: %+v", stats)
	}
	testsupport.Check(t, m.Start())
	<-ready
	stats, err = simfs.Stats(m)
	testsupport.Check(t, err)
	if stats.UsedBytes != 2048 || stats.FreeBytes != 0 {
		t.Fatalf("full stats: %+v", stats)
	}
	testsupport.Check(t, simfs.SetReadOnly(m, true))
	resume <- struct{}{}
	if err := <-blocked; !errors.Is(err, syscall.EROFS) {
		t.Fatalf("open descriptor ignored read-only mode: %v", err)
	}
	stats, err = simfs.Stats(m)
	testsupport.Check(t, err)
	if !stats.ReadOnly {
		t.Fatalf("read-only stats: %+v", stats)
	}
	testsupport.Check(t, simfs.SetReadOnly(m, false))
	resume <- struct{}{}
	testsupport.Wait(t, m)
	testsupport.Check(t, m.Destroy())
}

func TestPublicStorageOtherCapacityAndLatency(t *testing.T) {
	m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
		f, err := os.OpenFile("larger", os.O_CREATE|os.O_RDWR, 0o600)
		testsupport.Check(t, err)
		defer f.Close()
		start := time.Now()
		_, err = f.WriteAt([]byte("sparse"), 2048)
		testsupport.Check(t, err)
		if elapsed := time.Since(start); elapsed < 100*time.Nanosecond {
			t.Fatalf("storage write latency: %s", elapsed)
		}
		start = time.Now()
		_, err = f.Stat()
		testsupport.Check(t, err)
		if elapsed := time.Since(start); elapsed != 0 {
			t.Fatalf("metadata query advanced simulated time: %s", elapsed)
		}
	}})
	testsupport.Check(t, err)
	config := simfs.DefaultConfig()
	config.CapacityBytes = 4 * 1024
	config.Latency = 100 * time.Nanosecond
	testsupport.Check(t, simfs.ConfigureMachine(m, config))
	testsupport.Check(t, m.Start())
	testsupport.Wait(t, m)
	stats, err := simfs.Stats(m)
	testsupport.Check(t, err)
	if stats.TotalBytes != 4096 || stats.UsedBytes != 2048 || stats.FreeBytes != 2048 {
		t.Fatalf("sparse write stats: %+v", stats)
	}
	testsupport.Check(t, m.Destroy())
}

func TestPublicStorageEffectiveIdentity(t *testing.T) {
	m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
		f, err := os.OpenFile("owned", os.O_CREATE|os.O_RDWR, 0o660)
		testsupport.Check(t, err)
		defer f.Close()
		info, err := f.Stat()
		testsupport.Check(t, err)
		stat := info.Sys().(*syscall.Stat_t)
		if stat.Uid != 77 || stat.Gid != 88 || stat.Mode != 0o100640 {
			t.Fatalf("created identity/mode: %+v", stat)
		}
		if _, err := os.OpenFile("/etc/hosts", os.O_WRONLY, 0); !errors.Is(err, syscall.EACCES) {
			t.Fatalf("write to root-owned hosts: %v", err)
		}
		testsupport.Check(t, os.Chmod("owned", 0o600))
		if err := os.Chown("owned", 1, 2); !errors.Is(err, syscall.EPERM) {
			t.Fatalf("non-root chown: %v", err)
		}
	}})
	testsupport.Check(t, err)
	config := simfs.DefaultConfig()
	config.UID, config.GID = 77, 88
	testsupport.Check(t, simfs.ConfigureMachine(m, config))
	testsupport.Check(t, m.Start())
	testsupport.Wait(t, m)
	testsupport.Check(t, m.Destroy())
}

func TestPublicStorageRestoreAboveCapacity(t *testing.T) {
	source := testsupport.Start(t, func() {
		f, err := os.OpenFile("large", os.O_CREATE|os.O_RDWR, 0o600)
		testsupport.Check(t, err)
		_, err = f.Write(make([]byte, 2048))
		testsupport.Check(t, err)
		testsupport.Check(t, f.Sync())
		testsupport.Check(t, f.Close())
		dir, err := os.Open(".")
		testsupport.Check(t, err)
		testsupport.Check(t, dir.Sync())
		testsupport.Check(t, dir.Close())
		gosim.CrashSelf()
	})
	testsupport.Wait(t, source)
	iterator, err := simfs.CrashStates(source, simfs.EnumerateOptions{MaxStates: 16})
	testsupport.Check(t, err)
	snapshot, ok, err := iterator.Next()
	testsupport.Check(t, err)
	if !ok {
		t.Fatal("missing durable crash state")
	}
	testsupport.Check(t, iterator.Close())
	testsupport.Check(t, source.Destroy())
	target, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
		data, err := os.ReadFile("large")
		testsupport.Check(t, err)
		if len(data) != 2048 {
			t.Fatalf("over-capacity image lost readable data: %d", len(data))
		}
		if err := os.WriteFile("denied", []byte("x"), 0o600); !errors.Is(err, syscall.EROFS) {
			t.Fatalf("restore changed target read-only policy: %v", err)
		}
	}})
	testsupport.Check(t, err)
	config := simfs.DefaultConfig()
	config.CapacityBytes = 2048
	testsupport.Check(t, simfs.ConfigureMachine(target, config))
	testsupport.Check(t, simfs.SetReadOnly(target, true))
	testsupport.Check(t, simfs.Restore(target, snapshot))
	testsupport.Check(t, snapshot.Close())
	stats, err := simfs.Stats(target)
	testsupport.Check(t, err)
	if stats.TotalBytes != 2048 || stats.UsedBytes != 3072 || stats.FreeBytes != 0 || !stats.ReadOnly {
		t.Fatalf("restored over-capacity policy/stats: %+v", stats)
	}
	testsupport.Check(t, target.Start())
	testsupport.Wait(t, target)
	testsupport.Check(t, simfs.SetReadOnly(target, false))
	testsupport.Check(t, target.SetMain(func() {
		f, err := os.OpenFile("large", os.O_RDWR, 0)
		testsupport.Check(t, err)
		testsupport.Check(t, f.Truncate(0))
		testsupport.Check(t, f.Close())
		testsupport.Check(t, os.WriteFile("new", []byte("x"), 0o600))
	}))
	testsupport.Check(t, target.Restart())
	testsupport.Wait(t, target)
	stats, err = simfs.Stats(target)
	testsupport.Check(t, err)
	if stats.UsedBytes != 2048 || stats.FreeBytes != 0 || stats.ReadOnly {
		t.Fatalf("post-shrink stats: %+v", stats)
	}
	testsupport.Check(t, target.Destroy())
}
