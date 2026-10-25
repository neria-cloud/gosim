package model

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestSymlinkMetadataCrashCloneAndCollection(t *testing.T) {
	fs, dir := pathFixture(t)
	file := pathFile(t, fs, dir, "target", 0o600)
	pathCheck(t, fs.Symlink(dir, "link", "target", Identity{}))
	link, err := fs.Lstat(dir, "link", Identity{})
	pathCheck(t, err)
	if !link.IsSymlink || link.Mode != 0o777 || link.Nlink != 1 || link.Size != 6 {
		t.Fatal(link)
	}
	pathCheck(t, fs.Chown(link.Inode, 123, 456, Identity{}))
	fs.FlushEverything()
	image := fs.FreezeCrash()
	defer image.Release()
	clone := image.Clone(false, nil)
	defer clone.Release()
	pathCheck(t, fs.Rename(dir, "link", dir, "moved", Identity{}))
	pathCheck(t, fs.Remove(dir, "moved", false, Identity{}))
	fs.Sync(dir)
	if _, err := fs.Statfd(link.Inode); !errors.Is(err, syscall.ENOENT) {
		t.Fatal("unlinked symlink retained", err)
	}
	got, err := clone.Readlink(dir, "link", Identity{})
	pathCheck(t, err)
	if got != "target" {
		t.Fatal(got)
	}
	stat, err := clone.Lstat(dir, "link", Identity{})
	pathCheck(t, err)
	if stat.UID != 123 || stat.GID != 456 {
		t.Fatal("durable ownership lost", stat)
	}
	stat, err = clone.Stat(dir, "link", Identity{})
	pathCheck(t, err)
	if stat.Inode != file || stat.UID != 0 {
		t.Fatal("link owner changed target", stat)
	}
	pathCheck(t, clone.Chown(link.Inode, 789, -1, Identity{}))
	again := image.Clone(false, nil)
	defer again.Release()
	stat, err = again.Lstat(dir, "link", Identity{})
	pathCheck(t, err)
	if stat.UID != 123 || stat.GID != 456 {
		t.Fatal("clone metadata leaked into image", stat)
	}
	entries, err := again.ReadDir(dir, Identity{})
	pathCheck(t, err)
	if len(entries) != 2 || entries[0].Name != "link" || !entries[0].IsSymlink {
		t.Fatal(entries)
	}
	fd, err := again.OpenFile(dir, "target", os.O_RDONLY, 0, Identity{})
	pathCheck(t, err)
	pathCheck(t, again.Remove(dir, "target", false, Identity{}))
	stat, err = again.Statfd(fd)
	pathCheck(t, err)
	if stat.Nlink != 0 {
		t.Fatal(stat)
	}
	pathCheck(t, again.CloseFile(fd))
}
