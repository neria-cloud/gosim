package behavior_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func flockCheck(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func flockOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	flockCheck(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func flockTry(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }

func flockBusy(t *testing.T, f *os.File) {
	t.Helper()
	if err := flockTry(f); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("want contention, got %v", err)
	}
}

func TestFlockIndependentOpenExclusion(t *testing.T) {
	path := filepath.Join(descriptorTempDir(t), "owner")
	a, b, c := flockOpen(t, path), flockOpen(t, path), flockOpen(t, path)
	flockCheck(t, flockTry(a))
	flockCheck(t, unix.Flock(int(a.Fd()), unix.LOCK_EX|unix.LOCK_NB))
	flockBusy(t, b)
	flockCheck(t, unix.Flock(int(b.Fd()), unix.LOCK_UN))
	flockCheck(t, b.Close())
	flockBusy(t, c)
	flockCheck(t, syscall.Flock(int(a.Fd()), syscall.LOCK_UN|syscall.LOCK_NB))
	flockCheck(t, flockTry(c))
	flockCheck(t, a.Close())
	d := flockOpen(t, path)
	flockBusy(t, d)
	flockCheck(t, c.Close())
	flockCheck(t, flockTry(d))
}

func TestFlockBlockingUncontended(t *testing.T) {
	path := filepath.Join(descriptorTempDir(t), "owner")
	a, b := flockOpen(t, path), flockOpen(t, path)
	flockCheck(t, syscall.Flock(int(a.Fd()), syscall.LOCK_EX))
	flockCheck(t, syscall.Flock(int(a.Fd()), syscall.LOCK_EX))
	flockBusy(t, b)
	flockCheck(t, a.Close())
	flockCheck(t, syscall.Flock(int(b.Fd()), syscall.LOCK_EX))
}

func TestFlockContentionBeforeFactory(t *testing.T) {
	base := descriptorTempDir(t)
	for _, heldName := range []string{"system.lock", "wal.lock"} {
		t.Run(heldName, func(t *testing.T) {
			root := filepath.Join(base, heldName)
			flockCheck(t, os.Mkdir(root, 0o700))
			held := flockOpen(t, filepath.Join(root, heldName))
			flockCheck(t, flockTry(held))
			factories := 0
			startup := func() error {
				var acquired []*os.File
				defer func() {
					for _, f := range acquired {
						_ = f.Close()
					}
				}()
				for _, name := range []string{"system.lock", "wal.lock"} {
					f := flockOpen(t, filepath.Join(root, name))
					acquired = append(acquired, f)
					if err := flockTry(f); err != nil {
						return err
					}
				}
				factories++
				return os.WriteFile(filepath.Join(root, "metadata"), []byte("created"), 0o600)
			}
			if err := startup(); !errors.Is(err, syscall.EWOULDBLOCK) {
				t.Fatal(err)
			}
			if factories != 0 {
				t.Fatal("contended startup invoked factory")
			}
			if _, err := os.Stat(filepath.Join(root, "metadata")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("contended startup created metadata", err)
			}
			if heldName == "wal.lock" { // a failed second acquisition must release the first
				partial := flockOpen(t, filepath.Join(root, "system.lock"))
				flockCheck(t, flockTry(partial))
				flockCheck(t, partial.Close())
			}
			flockCheck(t, held.Close())
			flockCheck(t, startup())
			if factories != 1 {
				t.Fatal(factories)
			}
		})
	}
}

func TestFlockApplicationCloseRetryAndLeaseTransfer(t *testing.T) {
	root := descriptorTempDir(t)
	system, wal := flockOpen(t, filepath.Join(root, "system")), flockOpen(t, filepath.Join(root, "wal"))
	flockCheck(t, flockTry(system))
	flockCheck(t, flockTry(wal))
	contender := flockOpen(t, filepath.Join(root, "wal"))
	// Transfer the existing open description, never unlock/reopen to transfer.
	lease := wal
	wal = nil
	flockCheck(t, system.Close())
	flockBusy(t, contender)
	attempts := 0
	applicationClose := func() error {
		attempts++
		if attempts == 1 {
			return errors.New("retry application teardown")
		}
		return lease.Close()
	}
	if applicationClose() == nil {
		t.Fatal("missing injected Close failure")
	}
	flockBusy(t, contender)
	// Returning the same lease to its original owner also preserves exclusion.
	wal = lease
	if wal != lease {
		t.Fatal("lease identity changed")
	}
	flockBusy(t, contender)
	flockCheck(t, applicationClose())
	flockCheck(t, flockTry(contender))
	if attempts != 2 {
		t.Fatal(attempts)
	}
}

func TestFlockIdentityAfterRename(t *testing.T) {
	root := descriptorTempDir(t)
	path, moved := filepath.Join(root, "lock"), filepath.Join(root, "old")
	a := flockOpen(t, path)
	flockCheck(t, flockTry(a))
	flockCheck(t, os.Rename(path, moved))
	b := flockOpen(t, moved)
	flockBusy(t, b)
	flockCheck(t, os.Symlink(moved, filepath.Join(root, "alias")))
	alias := flockOpen(t, filepath.Join(root, "alias"))
	flockBusy(t, alias)
	replacement := flockOpen(t, path)
	flockCheck(t, flockTry(replacement))
	held, err := a.Stat()
	flockCheck(t, err)
	named, err := replacement.Stat()
	flockCheck(t, err)
	if os.SameFile(held, named) {
		t.Fatal("replacement retained held inode")
	}
	flockCheck(t, os.Remove(moved))
	flockBusy(t, b)
	flockCheck(t, a.Close())
	flockCheck(t, flockTry(b))
}
