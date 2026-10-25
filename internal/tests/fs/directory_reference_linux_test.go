//go:build linux && !sim

package behavior_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// Native-only reference for deliberately unsupported status/seek operations.
func TestGS01NativeLinuxDescriptorReference(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "file")
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_NONBLOCK, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil || flags&unix.O_NONBLOCK == 0 {
		t.Fatal(flags, err)
	}
	dir, err := os.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if _, err := unix.Seek(int(dir.Fd()), 0, unix.SEEK_CUR); err != nil {
		t.Fatal(err)
	}
	if _, err := unix.Openat(int(dir.Fd()), "file", unix.O_CREAT|unix.O_DIRECTORY, 0o600); !errors.Is(err, unix.EINVAL) {
		t.Fatal("requires Linux >=6.4 contract", err)
	}
	if os.Geteuid() != 0 {
		if err := os.Chmod(path, 0o400); err != nil {
			t.Fatal(err)
		}
		if opened, err := unix.Open(path, unix.O_RDONLY|unix.O_TRUNC, 0); !errors.Is(err, unix.EACCES) {
			if err == nil {
				unix.Close(opened)
			}
			t.Fatal("truncate without write access", err)
		}
		locked := filepath.Join(base, "locked")
		if err := os.Mkdir(locked, 0o700); err != nil {
			t.Fatal(err)
		}
		child := filepath.Join(locked, "file")
		if err := os.WriteFile(child, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(locked, 0o500); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(locked, 0o700)
		if err := unix.Renameat(unix.AT_FDCWD, child, unix.AT_FDCWD, child); err != nil {
			t.Fatal("same-inode rename requires parent write", err)
		}
		if err := unix.Renameat(unix.AT_FDCWD, child, unix.AT_FDCWD, filepath.Join(base, "other")); !errors.Is(err, unix.EACCES) {
			t.Fatal(err)
		}
		if err := unix.Unlinkat(unix.AT_FDCWD, child, unix.AT_REMOVEDIR); !errors.Is(err, unix.EACCES) {
			t.Fatal("type error preceded permission", err)
		}
	}
}

func TestGS01NativeLinuxRemovedDirectoryParent(t *testing.T) {
	base := t.TempDir()
	name := filepath.Join(base, "removed")
	if err := os.Mkdir(name, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Openat(int(dir.Fd()), "..", unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		t.Fatal(err)
	}
	parent := os.NewFile(uintptr(fd), "parent")
	defer parent.Close()
	held, err := parent.Stat()
	if err != nil {
		t.Fatal(err)
	}
	named, err := os.Stat(base)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(held, named) {
		t.Fatal("removed-directory parent changed")
	}
	if err := unix.Mkdirat(int(dir.Fd()), "new", 0o700); !errors.Is(err, unix.ENOENT) {
		t.Fatal(err)
	}
}
