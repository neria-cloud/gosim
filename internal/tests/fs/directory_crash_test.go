//go:build sim

package behavior_test

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func gs01SyncDir(t *testing.T, name string) {
	t.Helper()
	f, err := os.Open(name)
	testsupport.Check(t, err)
	testsupport.Check(t, f.Sync())
	testsupport.Check(t, f.Close())
}

func gs01Exists(t *testing.T, name string) bool {
	t.Helper()
	_, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	testsupport.Check(t, err)
	return true
}

func gs01EachCrash(t *testing.T, setup, inspect func()) int {
	t.Helper()
	source := testsupport.Start(t, func() { setup(); gosim.CrashSelf() })
	testsupport.Wait(t, source)
	defer func() { testsupport.Check(t, source.Destroy()) }()
	it, err := simfs.CrashStates(source, simfs.EnumerateOptions{MaxStates: 128})
	testsupport.Check(t, err)
	defer func() { testsupport.Check(t, it.Close()) }()
	count := 0
	for {
		snapshot, ok, err := it.Next()
		testsupport.Check(t, err)
		if !ok {
			break
		}
		count++
		restored, err := gosim.NewMachine(gosim.MachineConfig{Main: inspect})
		testsupport.Check(t, err)
		testsupport.Check(t, simfs.Restore(restored, snapshot))
		testsupport.Check(t, snapshot.Close())
		testsupport.Check(t, restored.Start())
		testsupport.Wait(t, restored)
		testsupport.Check(t, restored.Destroy())
	}
	if count == 0 {
		t.Fatal("no crash states")
	}
	return count
}

func TestGS01CrashDirectoryTopology(t *testing.T) {
	seen := map[string]bool{}
	count := gs01EachCrash(t, func() {
		testsupport.Check(t, os.MkdirAll("A/B", 0o700))
		gs01SyncDir(t, "A")
		gs01SyncDir(t, ".")
		testsupport.Check(t, os.Rename("A/B", "B"))
		testsupport.Check(t, os.Rename("A", "B/A"))
	}, func() {
		a, b, ba := gs01Exists(t, "A"), gs01Exists(t, "B"), gs01Exists(t, "B/A")
		switch {
		case a && !b && !ba:
			seen["original"] = true
		case a && b && !ba:
			seen["first"] = true
		case !a && b && ba:
			seen["both"] = true
		default:
			t.Fatal("invalid recovered namespace", a, b, ba)
		}
	})
	if count != 3 || len(seen) != 3 {
		t.Fatal(count, seen)
	}
}

func TestGS01CrashChildSyncAndSymlink(t *testing.T) {
	missing, present := false, false
	gs01EachCrash(t, func() {
		testsupport.Check(t, os.Mkdir("P", 0o700))
		f, err := os.Create("P/f")
		testsupport.Check(t, err)
		testsupport.Check(t, f.Close())
		testsupport.Check(t, os.Symlink("f", "P/link"))
		gs01SyncDir(t, "P")
	}, func() {
		if !gs01Exists(t, "P") {
			missing = true
			return
		}
		present = true
		info, err := os.Lstat("P/link")
		testsupport.Check(t, err)
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("link type lost")
		}
		target, err := os.Readlink("P/link")
		testsupport.Check(t, err)
		if target != "f" {
			t.Fatal(target)
		}
		data, err := os.ReadFile("P/link")
		testsupport.Check(t, err)
		if len(data) != 0 {
			t.Fatal(data)
		}
	})
	if !missing || !present {
		t.Fatal("child sync published parent", missing, present)
	}
}

func TestGS01CrashMovedDirectorySync(t *testing.T) {
	for _, child := range []bool{false, true} {
		name := "alone"
		if child {
			name = "dependent-child"
		}
		t.Run(name, func(t *testing.T) {
			old, new := false, false
			gs01EachCrash(t, func() {
				testsupport.Check(t, os.Mkdir("D", 0o700))
				testsupport.Check(t, os.Mkdir("E", 0o700))
				gs01SyncDir(t, ".")
				testsupport.Check(t, os.Rename("D", "moved"))
				if child {
					testsupport.Check(t, os.Rename("E", "moved/child"))
				}
				gs01SyncDir(t, "moved")
			}, func() {
				a, b := gs01Exists(t, "D"), gs01Exists(t, "moved")
				old = old || a
				new = new || b
				if child && (a || !b || !gs01Exists(t, "moved/child")) {
					t.Fatal("dependent topology lost")
				}
			})
			if !new || !child && !old {
				t.Fatal("wrong move durability", old, new)
			}
		})
	}
}

func TestGS01CrashEmptyDirectoryRemovalAndReplacement(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "remove"
		if replace {
			name = "replace"
		}
		t.Run(name, func(t *testing.T) {
			changed, unchanged := false, false
			count := gs01EachCrash(t, func() {
				testsupport.Check(t, os.Mkdir("X", 0o700))
				testsupport.Check(t, os.Mkdir("S", 0o700))
				f, err := os.Create("X/child")
				testsupport.Check(t, err)
				testsupport.Check(t, f.Close())
				gs01SyncDir(t, "X")
				gs01SyncDir(t, ".")
				testsupport.Check(t, os.Remove("X/child"))
				if replace {
					testsupport.Check(t, unix.Renameat(unix.AT_FDCWD, "S", unix.AT_FDCWD, "X"))
				} else {
					testsupport.Check(t, os.Remove("X"))
				}
			}, func() {
				hasX, hasChild := gs01Exists(t, "X"), gs01Exists(t, "X/child")
				mutated := !hasX
				if replace {
					mutated = !gs01Exists(t, "S")
				}
				if mutated && hasChild {
					t.Fatal("deleted child revived in changed directory")
				}
				changed = changed || mutated
				unchanged = unchanged || hasChild
			})
			if count != 3 || !changed || !unchanged {
				t.Fatal(count, changed, unchanged)
			}
		})
	}
}

func TestGS01CrashTopologyLeavesUnrelatedChangesUncertain(t *testing.T) {
	siblingOld, siblingNew, fileOld, fileNew, dataOld, dataNew := false, false, false, false, false, false
	gs01EachCrash(t, func() {
		for _, name := range []string{"D", "E", "S", "U"} {
			testsupport.Check(t, os.Mkdir(name, 0o700))
		}
		for _, name := range []string{"U/from", "U/data"} {
			f, err := os.Create(name)
			testsupport.Check(t, err)
			_, err = f.WriteString("keep")
			testsupport.Check(t, err)
			testsupport.Check(t, f.Sync())
			testsupport.Check(t, f.Close())
		}
		gs01SyncDir(t, "U")
		gs01SyncDir(t, ".")
		testsupport.Check(t, os.Rename("S", "sibling"))
		testsupport.Check(t, os.Rename("U/from", "U/to"))
		f, err := os.OpenFile("U/data", os.O_WRONLY, 0)
		testsupport.Check(t, err)
		_, err = f.WriteString("next")
		testsupport.Check(t, err)
		testsupport.Check(t, f.Close())
		testsupport.Check(t, os.Rename("D", "moved"))
		testsupport.Check(t, os.Rename("E", "moved/child"))
		gs01SyncDir(t, "moved")
	}, func() {
		if !gs01Exists(t, "moved/child") {
			t.Fatal("required topology missing")
		}
		siblingOld = siblingOld || gs01Exists(t, "S")
		siblingNew = siblingNew || gs01Exists(t, "sibling")
		fileOld = fileOld || gs01Exists(t, "U/from")
		fileNew = fileNew || gs01Exists(t, "U/to")
		data, err := os.ReadFile("U/data")
		testsupport.Check(t, err)
		dataOld = dataOld || string(data) == "keep"
		dataNew = dataNew || string(data) == "next"
	})
	if !(siblingOld && siblingNew && fileOld && fileNew && dataOld && dataNew) {
		t.Fatal("unrelated work became durable", siblingOld, siblingNew, fileOld, fileNew, dataOld, dataNew)
	}
}
