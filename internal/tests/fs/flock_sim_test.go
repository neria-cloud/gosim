//go:build sim

package behavior_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	"github.com/neria-cloud/gosim/rt"
)

func TestFlockSelectedHostMappingLifetime(t *testing.T) {
	path := filepath.Join(descriptorTempDir(t), "lock")
	a, b := flockOpen(t, path), flockOpen(t, path)
	_, err := a.WriteString("data")
	flockCheck(t, err)
	flockCheck(t, flockTry(a))
	data, err := unix.Mmap(int(a.Fd()), 0, 4096, unix.PROT_READ, unix.MAP_SHARED)
	flockCheck(t, err)
	flockCheck(t, a.Close())
	switch rt.Info().HostOS {
	case "linux":
		flockBusy(t, b)
	case "darwin":
		flockCheck(t, flockTry(b))
	default:
		t.Fatal("missing executable host identity")
	}
	flockCheck(t, unix.Munmap(data))
	flockCheck(t, flockTry(b))
}

func TestFlockOFDProbeFallback(t *testing.T) {
	// etcd performs this exact capability probe before choosing its flock path.
	before := syscall.Flock_t{Type: syscall.F_RDLCK}
	lock := before
	if err := syscall.FcntlFlock(0, unix.F_OFD_GETLK, &lock); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
	if lock != before {
		t.Fatal("OFD probe fabricated a lock result")
	}
	f := flockOpen(t, filepath.Join(descriptorTempDir(t), "fallback"))
	flockCheck(t, flockTry(f))
	flockBusy(t, flockOpen(t, f.Name()))
}

func TestFlockCrashReacquireAndRestoreIsolation(t *testing.T) {
	deferred := false
	source := testsupport.Start(t, func() {
		f, err := os.OpenFile("lock", os.O_CREATE|os.O_RDWR, 0o600)
		testsupport.Check(t, err)
		_, err = f.WriteString("durable")
		testsupport.Check(t, err)
		testsupport.Check(t, f.Sync())
		dir, err := os.Open(".")
		testsupport.Check(t, err)
		testsupport.Check(t, dir.Sync())
		testsupport.Check(t, dir.Close())
		testsupport.Check(t, flockTry(f))
		_, err = unix.Mmap(int(f.Fd()), 0, 7, unix.PROT_READ, unix.MAP_SHARED)
		testsupport.Check(t, err)
		defer func() { deferred = true; _ = f.Close() }()
		gosim.CrashSelf()
	})
	testsupport.Wait(t, source)
	if deferred {
		t.Fatal("crash executed application defer")
	}
	it, err := simfs.CrashStates(source, simfs.EnumerateOptions{MaxStates: 2})
	testsupport.Check(t, err)
	snapshot, ok, err := it.Next()
	testsupport.Check(t, err)
	if !ok {
		t.Fatal("missing durable crash image")
	}
	testsupport.Check(t, it.Close())
	inspect := func() {
		f, err := os.OpenFile("lock", os.O_RDWR, 0)
		testsupport.Check(t, err)
		defer f.Close()
		testsupport.Check(t, flockTry(f))
		data, err := os.ReadFile("lock")
		testsupport.Check(t, err)
		if string(data) != "durable" {
			t.Fatal("crash lost synced data", string(data))
		}
	}
	testsupport.Check(t, source.SetMain(inspect))
	testsupport.Check(t, source.Restart())
	testsupport.Wait(t, source)
	testsupport.Check(t, source.Destroy())
	ready, release := make(chan bool, 2), make(chan bool)
	var copies []gosim.Machine
	for range 2 {
		m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
			f, err := os.OpenFile("lock", os.O_RDWR, 0)
			testsupport.Check(t, err)
			defer f.Close()
			testsupport.Check(t, flockTry(f))
			ready <- true
			<-release
		}})
		testsupport.Check(t, err)
		testsupport.Check(t, simfs.Restore(m, snapshot))
		testsupport.Check(t, m.Start())
		copies = append(copies, m)
	}
	<-ready
	<-ready // both private copies may hold their lock simultaneously
	close(release)
	for _, m := range copies {
		testsupport.Wait(t, m)
		testsupport.Check(t, m.Destroy())
	}
	testsupport.Check(t, snapshot.Close())
}

func TestFlockCloseWaitsForDescriptorDestruction(t *testing.T) {
	m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
		f, err := os.OpenFile("lock", os.O_CREATE|os.O_RDWR, 0o600)
		testsupport.Check(t, err)
		_, err = f.WriteString("x")
		testsupport.Check(t, err)
		_, err = f.Seek(0, 0)
		testsupport.Check(t, err)
		other, err := os.Open("lock")
		testsupport.Check(t, err)
		defer other.Close()
		testsupport.Check(t, flockTry(f))
		started, joined := make(chan bool), make(chan error, 1)
		go func() {
			started <- true
			buf := make([]byte, 1)
			_, err := f.Read(buf)
			joined <- err
		}()
		<-started
		// Virtual time advances only after the read is waiting on its configured
		// 10ms scalar completion; no host sleeps or wall-clock ordering involved.
		time.Sleep(time.Millisecond)
		testsupport.Check(t, f.Close())
		flockBusy(t, other)
		testsupport.Check(t, <-joined)
		testsupport.Check(t, flockTry(other))
	}})
	testsupport.Check(t, err)
	testsupport.Check(t, simfs.ConfigureMachine(m, simfs.Config{WorkingDir: "/app", Latency: 10 * time.Millisecond}))
	testsupport.Check(t, m.Start())
	testsupport.Wait(t, m)
	testsupport.Check(t, m.Destroy())
}
