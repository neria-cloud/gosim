//go:build (linux || darwin) && !sim

package behavior_test

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestFlockNativeGoDuplicateLifetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a, b := flockOpen(t, path), flockOpen(t, path)
	flockCheck(t, flockTry(a))
	alias, err := syscall.Dup(int(a.Fd()))
	flockCheck(t, err)
	defer func() {
		if alias >= 0 {
			_ = syscall.Close(alias)
		}
	}()
	flockCheck(t, a.Close())
	flockBusy(t, b)
	flockCheck(t, syscall.Flock(alias, syscall.LOCK_UN))
	flockCheck(t, flockTry(b))
	flockCheck(t, syscall.Flock(int(b.Fd()), syscall.LOCK_UN))
	flockCheck(t, syscall.Flock(alias, syscall.LOCK_EX|syscall.LOCK_NB))
	flockBusy(t, b)
	flockCheck(t, syscall.Close(alias))
	alias = -1
	flockCheck(t, flockTry(b))
}

func TestFlockNativeGoMappingLifetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a, b := flockOpen(t, path), flockOpen(t, path)
	flockCheck(t, a.Truncate(4096))
	flockCheck(t, flockTry(a))
	data, err := syscall.Mmap(int(a.Fd()), 0, 4096, syscall.PROT_READ, syscall.MAP_SHARED)
	flockCheck(t, err)
	defer func() {
		if data != nil {
			_ = syscall.Munmap(data)
		}
	}()
	flockCheck(t, a.Close())
	if runtime.GOOS == "linux" {
		flockBusy(t, b)
	} else {
		flockCheck(t, flockTry(b))
	}
	flockCheck(t, syscall.Munmap(data))
	data = nil
	flockCheck(t, flockTry(b))
}

func TestFlockNativeGoProcessExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "durable")
	f := flockOpen(t, path)
	_, err := f.WriteString("durable owner data")
	flockCheck(t, err)
	flockCheck(t, f.Sync())
	cmd := exec.Command(os.Args[0], "-test.run=^TestFlockNativeGoChild$")
	cmd.Env = append(os.Environ(), "GOSIM_FLOCK_CHILD="+path)
	output, err := cmd.StdoutPipe()
	flockCheck(t, err)
	cmd.Stderr = os.Stderr
	flockCheck(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(output).ReadString('\n')
		if err == nil && line != "locked\n" {
			err = fmt.Errorf("unexpected child response: %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		flockCheck(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("child did not acquire lock")
	}
	flockBusy(t, f)
	flockCheck(t, cmd.Process.Kill())
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed child returned success")
	}
	flockCheck(t, flockTry(f))
	data, err := os.ReadFile(path)
	flockCheck(t, err)
	if string(data) != "durable owner data" {
		t.Fatal("process exit lost data")
	}
}

func TestFlockNativeGoChild(t *testing.T) {
	path := os.Getenv("GOSIM_FLOCK_CHILD")
	if path == "" {
		t.Skip("child helper; run by TestFlockNativeGoProcessExit")
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	flockCheck(t, err)
	flockCheck(t, flockTry(f))
	_, err = syscall.Mmap(int(f.Fd()), 0, 4096, syscall.PROT_READ, syscall.MAP_SHARED)
	flockCheck(t, err)
	fmt.Println("locked")
	time.Sleep(time.Hour) // parent owns the child and terminates it after the handshake
	runtime.KeepAlive(f)
}
