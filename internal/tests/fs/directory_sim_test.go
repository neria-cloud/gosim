//go:build sim

package behavior_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
)

func TestGS01SimulatorDescriptorLimitsAndReadOnly(t *testing.T) {
	base := descriptorTempDir(t)
	name := filepath.Join(base, "file")
	if err := os.WriteFile(name, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := unix.FcntlInt(file.Fd(), unix.F_GETFL, 0); !errors.Is(err, unix.ENOSYS) {
		t.Fatal(err)
	}
	if fd, err := unix.Open(name, unix.O_RDONLY|unix.O_NONBLOCK, 0); !errors.Is(err, unix.EINVAL) {
		if err == nil {
			unix.Close(fd)
		}
		t.Fatal(err)
	}
	dir, err := os.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if _, err := unix.Seek(int(dir.Fd()), 0, unix.SEEK_CUR); !errors.Is(err, unix.EINVAL) {
		t.Fatal(err)
	}
	machine := gosim.CurrentMachine()
	if err := simfs.SetReadOnly(machine, true); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := simfs.SetReadOnly(machine, false); err != nil {
			t.Error(err)
		}
	}()
	for _, op := range []func() error{
		func() error { return os.Mkdir(base, 0o700) }, // deliberate early EROFS, not EEXIST
		func() error {
			f, err := os.OpenFile(name, os.O_CREATE|os.O_RDONLY, 0o600)
			if f != nil {
				f.Close()
			}
			return err
		},
		func() error { _, err := file.WriteString("bad"); return err },
	} {
		if err := op(); !errors.Is(err, unix.EROFS) {
			t.Fatal(err)
		}
	}
	position, err := file.Seek(0, 1)
	if err != nil || position != 0 {
		t.Fatal(position, err)
	}
	data, err := os.ReadFile(name)
	if err != nil || string(data) != "keep" {
		t.Fatal(string(data), err)
	}
	if err := simfs.SetReadOnly(machine, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("retry"), 0o600); err != nil {
		t.Fatal(err)
	}
}
