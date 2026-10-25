package model

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestNamespaceMutationValidationAndHeldReplacement(t *testing.T) {
	fs, dir := pathFixture(t)
	for _, name := range []string{"A", "A/child", "empty", "full"} {
		pathCheck(t, fs.Mkdir(dir, name, 0o755, Identity{}))
	}
	file := pathFile(t, fs, dir, "file", 0o644)
	pathFile(t, fs, dir, "full/file", 0o644)
	pathCheck(t, fs.Symlink(dir, "link", "A", Identity{}))
	for _, tc := range []struct {
		from, to string
		want     error
	}{
		{"A", "A/child/new", syscall.EINVAL},
		{"A/child", "A", syscall.ENOTEMPTY},
		{"full/file", "full", syscall.ENOTEMPTY},
		{"file", "empty", syscall.EISDIR},
		{"empty", "file", syscall.ENOTDIR},
		{"A", "full", syscall.ENOTEMPTY},
		{"file/", "moved", syscall.ENOTDIR},
		{"file", "new/", syscall.ENOTDIR},
		{"link/", "moved", syscall.ENOTDIR},
		{".", "moved", syscall.EBUSY},
	} {
		before := len(fs.pendingOps.allOps())
		if err := fs.Rename(dir, tc.from, dir, tc.to, Identity{}); !errors.Is(err, tc.want) {
			t.Fatalf("rename %s -> %s: %v want %v", tc.from, tc.to, err, tc.want)
		}
		if len(fs.pendingOps.allOps()) != before {
			t.Fatal("failed rename mutated state")
		}
	}
	for _, tc := range []struct {
		name      string
		directory bool
		want      error
	}{
		{"A", false, syscall.EISDIR},
		{"file", true, syscall.ENOTDIR},
		{"link", true, syscall.ENOTDIR},
		{"link/", false, syscall.ENOTDIR},
		{"A", true, syscall.ENOTEMPTY},
		{".", true, syscall.EINVAL},
		{"..", true, syscall.ENOTEMPTY},
		{"/", true, syscall.EBUSY},
	} {
		if err := fs.Remove(dir, tc.name, tc.directory, Identity{}); !errors.Is(err, tc.want) {
			t.Fatalf("remove %s: %v want %v", tc.name, err, tc.want)
		}
	}
	held, err := fs.OpenFile(dir, "empty", os.O_RDONLY, 0, Identity{})
	pathCheck(t, err)
	pathCheck(t, fs.Rename(dir, "A", dir, "empty", Identity{}))
	if _, err := fs.Statfd(held); err != nil {
		t.Fatal("overwritten handle lost inode", err)
	}
	if err := fs.Mkdir(held, "new", 0o700, Identity{}); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	pathCheck(t, fs.CloseFile(held))
	old := pathFile(t, fs, dir, "destination", 0o644)
	oldFD, err := fs.OpenFile(dir, "destination", os.O_RDONLY, 0, Identity{})
	pathCheck(t, err)
	pathCheck(t, fs.Rename(dir, "file", dir, "destination", Identity{}))
	stat, err := fs.Stat(dir, "destination", Identity{})
	pathCheck(t, err)
	if stat.Inode != file || oldFD != old {
		t.Fatal(stat, oldFD, old)
	}
	stat, err = fs.Statfd(oldFD)
	pathCheck(t, err)
	if stat.Nlink != 0 {
		t.Fatal("overwritten open file still linked", stat)
	}
	pathCheck(t, fs.CloseFile(oldFD))
}

func TestSameInodeRenameDoesNotRequireParentWrite(t *testing.T) {
	fs, dir := pathFixture(t)
	owner := Identity{UID: 42, GID: 42}
	pathCheck(t, fs.Mkdir(dir, "locked", 0o500, owner))
	pathFile(t, fs, dir, "locked/file", 0o444)
	before := len(fs.pendingOps.allOps())
	pathCheck(t, fs.Rename(dir, "locked/file", dir, "locked/file", owner))
	if len(fs.pendingOps.allOps()) != before {
		t.Fatal("no-op emitted mutation")
	}
	if err := fs.Rename(dir, "locked/file", dir, "moved", owner); !errors.Is(err, syscall.EACCES) {
		t.Fatal(err)
	}
	if err := fs.Remove(dir, "locked/file", true, owner); !errors.Is(err, syscall.EACCES) {
		t.Fatal("type error preceded parent permission", err)
	}
}

func TestPreparedNamespaceInputsAreOwned(t *testing.T) {
	fs, _ := pathFixture(t)
	parts := []dirOpPart{{dir: 1, name: "before", file: 2}}
	chain, empty := []int{3, 4}, []int{5}
	op := fs.prepareDirOp(parts, 2, chain, empty)
	parts[0].name = "after"
	chain[0] = 9
	empty[0] = 9
	if op.parts[0].name != "before" || op.destinationChain[0] != 3 || op.emptyDirs[0] != 5 {
		t.Fatal("caller mutation changed pending operation")
	}
}

func TestCorruptDirectoryParentCycleIsAnInvariantFailure(t *testing.T) {
	for _, operation := range []string{"ancestors", "getpath"} {
		t.Run(operation, func(t *testing.T) {
			fs, dir := pathFixture(t)
			pathCheck(t, fs.Mkdir(dir, "cycle", 0o700, Identity{}))
			inode, err := fs.Getdirinode(dir, "cycle", Identity{})
			pathCheck(t, err)
			object, _ := fs.mem.getDir(inode)
			object.parent = inode // deliberately impossible internal state
			defer func() {
				if got := recover(); got != "filesystem directory topology cycle" {
					t.Fatalf("wrong invariant outcome: %v", got)
				}
			}()
			if operation == "ancestors" {
				_, _ = fs.directoryAncestorsInclusive(inode)
			} else {
				_, _ = fs.Getpath(inode)
			}
		})
	}
}
