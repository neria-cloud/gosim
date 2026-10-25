package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func pathCheck(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func pathFixture(t *testing.T) (*Filesystem, int) {
	t.Helper()
	fs := NewLinuxFilesystem()
	t.Cleanup(fs.Release)
	dir, err := fs.Getdirinode(RootInode, "/app", Identity{})
	pathCheck(t, err)
	return fs, dir
}

func pathFile(t *testing.T, fs *Filesystem, dir int, name string, mode uint32) int {
	t.Helper()
	inode, err := fs.OpenFile(dir, name, os.O_CREATE|os.O_RDWR, mode, Identity{})
	pathCheck(t, err)
	_, err = fs.Write(inode, 0, abi.ByteView{Ptr: []byte("keep")}, DefaultCapacityBytes)
	pathCheck(t, err)
	pathCheck(t, fs.CloseFile(inode))
	return inode
}

func TestPathComponentsAndFinalPolicy(t *testing.T) {
	fs, dir := pathFixture(t)
	pathCheck(t, fs.Mkdir(dir, "one", 0o755, Identity{}))
	pathCheck(t, fs.Mkdir(dir, "one/two", 0o755, Identity{}))
	want := pathFile(t, fs, dir, "one/data", 0o644)
	pathCheck(t, fs.Symlink(dir, "deep", "one/two", Identity{}))
	pathCheck(t, fs.Symlink(dir, "absolute", "/app/one/data", Identity{}))
	for _, name := range []string{"one/data", "one//./data", "deep/../data", "absolute", "/app/one/data"} {
		stat, err := fs.Stat(dir, name, Identity{})
		if err != nil || stat.Inode != want {
			t.Fatalf("stat %q: %+v, %v", name, stat, err)
		}
	}
	for _, tc := range []struct {
		path, name string
		inode      int
	}{
		{"/", "", RootInode}, {"///", "", RootInode}, {".", ".", dir}, {"..", "..", RootInode},
	} {
		entry, err := fs.resolve(dir, tc.path, resolveOptions{Final: resolveMutationEntry}, Identity{})
		if err != nil || entry.Name != tc.name || entry.Inode != tc.inode {
			t.Fatalf("%q: %+v, %v", tc.path, entry, err)
		}
	}
	link, err := fs.Lstat(dir, "deep", Identity{})
	pathCheck(t, err)
	if !link.IsSymlink || link.Size != int64(len("one/two")) {
		t.Fatal(link)
	}
	stat, err := fs.Lstat(dir, "deep/", Identity{})
	pathCheck(t, err)
	if !stat.IsDir {
		t.Fatal("trailing slash failed to traverse directory link")
	}
	for _, tc := range []struct {
		path string
		want error
	}{
		{"", syscall.ENOENT},
		{"missing/data", syscall.ENOENT},
		{"one/data/child", syscall.ENOTDIR},
		{"one/data/", syscall.ENOTDIR},
		{"x\x00y", syscall.EINVAL},
		{strings.Repeat("a", 256), syscall.ENAMETOOLONG},
		{strings.Repeat("/", 4096), syscall.ENAMETOOLONG},
	} {
		if _, err := fs.Stat(dir, tc.path, Identity{}); !errors.Is(err, tc.want) {
			t.Fatalf("%q: %v, want %v", tc.path, err, tc.want)
		}
	}
}

func TestCreateAndOpenFailureAtomicity(t *testing.T) {
	fs, dir := pathFixture(t)
	file := pathFile(t, fs, dir, "file", 0o644)
	pathCheck(t, fs.Mkdir(dir, "directory", 0o755, Identity{}))
	pathCheck(t, fs.Symlink(dir, "link", "file", Identity{}))
	pathCheck(t, fs.Symlink(dir, "dangling", "absent", Identity{}))
	for _, tc := range []struct {
		name    string
		flags   int
		options OpenOptions
		who     Identity
		want    error
	}{
		{"file", os.O_TRUNC, OpenOptions{Directory: true}, Identity{}, syscall.ENOTDIR},
		{"absent", os.O_CREATE, OpenOptions{Directory: true}, Identity{}, syscall.EINVAL},
		{"link", os.O_TRUNC, OpenOptions{NoFollow: true}, Identity{}, syscall.ELOOP},
		{"link", os.O_RDONLY, OpenOptions{Directory: true, NoFollow: true}, Identity{}, syscall.ENOTDIR},
		{"dangling", os.O_CREATE | os.O_EXCL | os.O_WRONLY, OpenOptions{}, Identity{}, syscall.EEXIST},
		{"file/", os.O_CREATE, OpenOptions{}, Identity{}, syscall.EISDIR},
		{"directory/", os.O_CREATE | os.O_EXCL, OpenOptions{}, Identity{}, syscall.EISDIR},
		{"dangling/", os.O_CREATE, OpenOptions{}, Identity{}, syscall.EISDIR},
		{"absent/", os.O_CREATE, OpenOptions{}, Identity{}, syscall.EISDIR},
		{"directory", os.O_TRUNC, OpenOptions{}, Identity{}, syscall.EISDIR},
		{"file", os.O_RDONLY | os.O_TRUNC, OpenOptions{}, Identity{UID: 1001}, syscall.EACCES},
	} {
		t.Run(fmt.Sprintf("%s-%x-%v", tc.name, tc.flags, tc.options), func(t *testing.T) {
			next, pending, handles := fs.mem.next, len(fs.pendingOps.allOps()), fs.GetInodeInfo(file).Handles
			fd, err := fs.OpenFileWithOptions(dir, tc.name, tc.flags, 0o600, tc.options, tc.who)
			if !errors.Is(err, tc.want) {
				if err == nil {
					fs.CloseFile(fd)
				}
				t.Fatalf("%v, want %v", err, tc.want)
			}
			if fs.mem.next != next || len(fs.pendingOps.allOps()) != pending || fs.GetInodeInfo(file).Handles != handles {
				t.Fatal("failed open mutated namespace/references")
			}
			buf := make([]byte, 4)
			if fs.Read(file, 0, abi.ByteView{Ptr: buf}) != 4 || string(buf) != "keep" {
				t.Fatal("failed open changed file")
			}
		})
	}
	for _, name := range []string{"file/", "link/", "directory/", ".", "..", "/"} {
		if err := fs.Mkdir(dir, name, 0o700, Identity{}); !errors.Is(err, syscall.EEXIST) {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := fs.Symlink(dir, name, "file", Identity{}); !errors.Is(err, syscall.EEXIST) {
			t.Fatalf("symlink %s: %v", name, err)
		}
	}
	if err := fs.Symlink(dir, "absent/", "file", Identity{}); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	pathCheck(t, fs.Mkdir(dir, "new/", 0o700, Identity{}))
	fd, err := fs.OpenFile(dir, "file", os.O_EXCL, 0, Identity{}) // EXCL without CREATE is ignored.
	pathCheck(t, err)
	pathCheck(t, fs.CloseFile(fd))
	fd, err = fs.OpenFile(dir, "file", os.O_RDONLY|os.O_TRUNC, 0, Identity{})
	pathCheck(t, err)
	pathCheck(t, fs.CloseFile(fd))
	stat, err := fs.Statfd(file)
	pathCheck(t, err)
	if stat.Size != 0 {
		t.Fatal("RDONLY|TRUNC did not truncate", stat)
	}
}

func TestOpenCreateSharesSymlinkBudgetWithPrefix(t *testing.T) {
	fs, dir := pathFixture(t)
	pathCheck(t, fs.Mkdir(dir, "real", 0o755, Identity{}))
	file := pathFile(t, fs, dir, "real/data", 0o644)
	pathCheck(t, fs.Symlink(dir, "real/leaf0", "leaf1", Identity{}))
	pathCheck(t, fs.Symlink(dir, "real/leaf1", "data", Identity{}))
	for n := 38; n >= 0; n-- {
		target := "real"
		if n < 38 {
			target = fmt.Sprintf("p%d", n+1)
		}
		pathCheck(t, fs.Symlink(dir, fmt.Sprintf("p%d", n), target, Identity{}))
	}
	for _, flags := range []int{os.O_RDONLY, os.O_CREATE | os.O_RDWR} {
		if fd, err := fs.OpenFile(dir, "p0/leaf0", flags, 0o600, Identity{}); !errors.Is(err, syscall.ELOOP) {
			if err == nil {
				fs.CloseFile(fd)
			}
			t.Fatalf("41 links flags=%x: %v", flags, err)
		}
		fd, err := fs.OpenFile(dir, "p1/leaf0", flags, 0o600, Identity{})
		pathCheck(t, err)
		if fd != file {
			t.Fatal("40 links resolved wrong inode", fd)
		}
		pathCheck(t, fs.CloseFile(fd))
	}
	pathCheck(t, fs.Symlink(dir, "loop", "loop", Identity{}))
	if _, err := fs.Stat(dir, "loop", Identity{}); !errors.Is(err, syscall.ELOOP) {
		t.Fatal(err)
	}
}

func TestUnlinkedDirectoryRejectsNewEntries(t *testing.T) {
	fs, dir := pathFixture(t)
	pathCheck(t, fs.Mkdir(dir, "held", 0o700, Identity{}))
	held, err := fs.OpenFile(dir, "held", os.O_RDONLY, 0, Identity{})
	pathCheck(t, err)
	pathCheck(t, fs.Remove(dir, "held", true, Identity{}))
	if _, err := fs.Statfd(held); err != nil {
		t.Fatal("held directory lost inode", err)
	}
	fs.Sync(held)
	for _, err := range []error{fs.Mkdir(held, "new", 0o700, Identity{}), fs.Symlink(held, "link", "x", Identity{})} {
		if !errors.Is(err, syscall.ENOENT) {
			t.Fatal(err)
		}
	}
	if _, err := fs.OpenFile(held, "file", os.O_CREATE, 0o600, Identity{}); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	if _, err := fs.Getpath(held); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	pathCheck(t, fs.CloseFile(held))
}
