package model

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func checkDirectoryGraph(t *testing.T, fs *Filesystem) {
	t.Helper()
	locations := make(map[int]int)
	for parent, obj := range fs.mem.objects {
		dir, ok := obj.(*backingDir)
		if !ok {
			continue
		}
		for name, inode := range dir.entries {
			if child, ok := fs.mem.getDir(inode); ok {
				locations[inode]++
				if locations[inode] > 1 || child.parent != parent || child.name != name || child.unlinked {
					t.Fatalf("inconsistent directory %d at %d/%s: %+v", inode, parent, name, child)
				}
			}
		}
		seen := make(map[int]bool)
		for inode := parent; inode != RootInode && inode != 0; {
			if seen[inode] {
				t.Fatalf("parent cycle through %d", inode)
			}
			seen[inode] = true
			ancestor, ok := fs.mem.getDir(inode)
			if !ok {
				t.Fatalf("missing parent %d", inode)
			}
			inode = ancestor.parent
		}
	}
}

func eachSmallCrash(t *testing.T, fs *Filesystem, check func(*Filesystem)) int {
	t.Helper()
	image := fs.FreezeCrash()
	defer image.Release()
	it := image.IterCrashes()
	defer it.Close()
	count := 0
	for {
		state, ok := it.Next()
		if !ok {
			break
		}
		count++
		if count > 256 {
			state.Release()
			t.Fatal("fixture exceeded bounded crash budget")
		}
		checkDirectoryGraph(t, state)
		check(state)
		state.Release()
	}
	if count == 0 {
		t.Fatal("empty crash enumeration")
	}
	return count
}

func TestDirectoryMovesCannotRecoverCycle(t *testing.T) {
	fs, dir := pathFixture(t)
	pathCheck(t, fs.Mkdir(dir, "A", 0o700, Identity{}))
	pathCheck(t, fs.Mkdir(dir, "A/B", 0o700, Identity{}))
	fs.FlushEverything()
	pathCheck(t, fs.Rename(dir, "A/B", dir, "B", Identity{}))
	pathCheck(t, fs.Rename(dir, "A", dir, "B/A", Identity{}))
	firstOnly, none, both := false, false, false
	count := eachSmallCrash(t, fs, func(state *Filesystem) {
		_, aErr := state.Stat(dir, "A", Identity{})
		_, bErr := state.Stat(dir, "B", Identity{})
		_, baErr := state.Stat(dir, "B/A", Identity{})
		none = none || aErr == nil && errors.Is(bErr, syscall.ENOENT)
		firstOnly = firstOnly || aErr == nil && bErr == nil && errors.Is(baErr, syscall.ENOENT)
		both = both || errors.Is(aErr, syscall.ENOENT) && baErr == nil
	})
	if count != 3 || !none || !firstOnly || !both {
		t.Fatalf("states=%d none=%t first=%t both=%t", count, none, firstOnly, both)
	}
}

func TestSyncChildDoesNotPublishUnsyncedMkdir(t *testing.T) {
	fs, dir := pathFixture(t)
	pathCheck(t, fs.Mkdir(dir, "P", 0o700, Identity{}))
	p, err := fs.Getdirinode(dir, "P", Identity{})
	pathCheck(t, err)
	fd, err := fs.OpenFile(p, "f", os.O_CREATE|os.O_RDWR, 0o600, Identity{})
	pathCheck(t, err)
	pathCheck(t, fs.CloseFile(fd))
	fs.Sync(p)
	missing, present := false, false
	eachSmallCrash(t, fs, func(state *Filesystem) {
		_, err := state.Stat(dir, "P", Identity{})
		if errors.Is(err, syscall.ENOENT) {
			missing = true
			return
		}
		pathCheck(t, err)
		present = true
		_, err = state.Stat(dir, "P/f", Identity{})
		pathCheck(t, err)
	})
	if !missing || !present {
		t.Fatal("child sync fixed parent namespace durability", missing, present)
	}
}

func TestDirectorySyncOwnLinkDependsOnFlushedChildMove(t *testing.T) {
	for _, childMove := range []bool{false, true} {
		t.Run(map[bool]string{false: "alone", true: "child-move"}[childMove], func(t *testing.T) {
			fs, dir := pathFixture(t)
			pathCheck(t, fs.Mkdir(dir, "D", 0o700, Identity{}))
			pathCheck(t, fs.Mkdir(dir, "E", 0o700, Identity{}))
			d, err := fs.Getdirinode(dir, "D", Identity{})
			pathCheck(t, err)
			fs.FlushEverything()
			pathCheck(t, fs.Rename(dir, "D", dir, "moved", Identity{}))
			if childMove {
				pathCheck(t, fs.Rename(dir, "E", d, "child", Identity{}))
			}
			fs.Sync(d)
			old, new := false, false
			eachSmallCrash(t, fs, func(state *Filesystem) {
				_, oldErr := state.Stat(dir, "D", Identity{})
				_, newErr := state.Stat(dir, "moved", Identity{})
				old = old || oldErr == nil
				new = new || newErr == nil
				if childMove {
					if oldErr == nil || newErr != nil {
						t.Fatal("dependent child move lost D topology", oldErr, newErr)
					}
					_, err := state.Stat(dir, "moved/child", Identity{})
					pathCheck(t, err)
				}
			})
			if !new || !childMove && !old {
				t.Fatal("wrong own-link uncertainty", old, new)
			}
		})
	}
}

func TestTopologySyncDoesNotPersistUnrelatedNamespaceOrData(t *testing.T) {
	fs, dir := pathFixture(t)
	for _, name := range []string{"D", "E", "S", "U"} {
		pathCheck(t, fs.Mkdir(dir, name, 0o700, Identity{}))
	}
	data := pathFile(t, fs, dir, "U/data", 0o600)
	pathFile(t, fs, dir, "U/from", 0o600)
	d, err := fs.Getdirinode(dir, "D", Identity{})
	pathCheck(t, err)
	fs.FlushEverything()
	pathCheck(t, fs.Rename(dir, "S", dir, "sibling-moved", Identity{}))
	pathCheck(t, fs.Rename(dir, "U/from", dir, "U/to", Identity{}))
	_, err = fs.Write(data, 0, abi.ByteView{Ptr: []byte("next")}, DefaultCapacityBytes)
	pathCheck(t, err)
	pathCheck(t, fs.Rename(dir, "D", dir, "moved", Identity{}))
	pathCheck(t, fs.Rename(dir, "E", d, "child", Identity{}))
	fs.Sync(d)
	siblingOld, siblingNew, fileOld, fileNew, bytesOld, bytesNew := false, false, false, false, false, false
	eachSmallCrash(t, fs, func(state *Filesystem) {
		_, err := state.Stat(dir, "moved/child", Identity{})
		pathCheck(t, err)
		_, err = state.Stat(dir, "S", Identity{})
		siblingOld = siblingOld || err == nil
		_, err = state.Stat(dir, "sibling-moved", Identity{})
		siblingNew = siblingNew || err == nil
		_, err = state.Stat(dir, "U/from", Identity{})
		fileOld = fileOld || err == nil
		_, err = state.Stat(dir, "U/to", Identity{})
		fileNew = fileNew || err == nil
		buf := make([]byte, 4)
		state.Read(data, 0, abi.ByteView{Ptr: buf})
		bytesOld = bytesOld || string(buf) == "keep"
		bytesNew = bytesNew || string(buf) == "next"
	})
	if !(siblingOld && siblingNew && fileOld && fileNew && bytesOld && bytesNew) {
		t.Fatal("unrelated work became durable", siblingOld, siblingNew, fileOld, fileNew, bytesOld, bytesNew)
	}
}

func TestRemovedOrReplacedDirectoryCannotRecoverChildren(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "remove", true: "replace"}[replace], func(t *testing.T) {
			fs, dir := pathFixture(t)
			pathCheck(t, fs.Mkdir(dir, "X", 0o700, Identity{}))
			pathCheck(t, fs.Mkdir(dir, "source", 0o700, Identity{}))
			pathFile(t, fs, dir, "X/child", 0o600)
			x, err := fs.Getdirinode(dir, "X", Identity{})
			pathCheck(t, err)
			fs.FlushEverything()
			pathCheck(t, fs.Remove(x, "child", false, Identity{}))
			if replace {
				pathCheck(t, fs.Rename(dir, "source", dir, "X", Identity{}))
			} else {
				pathCheck(t, fs.Remove(dir, "X", true, Identity{}))
			}
			removed, unchanged := false, false
			eachSmallCrash(t, fs, func(state *Filesystem) {
				obj, ok := state.mem.getDir(x)
				if !ok {
					t.Fatal("directory unexpectedly reclaimed")
				}
				if obj.unlinked {
					removed = true
					if len(obj.entries) != 0 {
						t.Fatal("removed directory recovered children")
					}
				} else if len(obj.entries) != 0 {
					unchanged = true
				}
			})
			if !removed || !unchanged {
				t.Fatal("lost valid uncertainty", removed, unchanged)
			}
		})
	}
}
