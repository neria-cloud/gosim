package behavior_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// These cases exercise the same public Go/x/sys entry points natively and in
// simulation. Linux-specific errno/flag precedence has separate coverage.
func TestDirectoryDescriptorRenameAndClose(t *testing.T) {
	if err := os.MkdirAll(os.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	original := filepath.Join(base, "original")
	moved := filepath.Join(base, "moved")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(original, 0o700))
	dir, err := os.Open(original)
	must(err)
	defer dir.Close()
	fd := int(dir.Fd())
	opened, err := unix.Openat(fd, ".", unix.O_RDONLY|unix.O_DIRECTORY, 0)
	must(err)
	defer unix.Close(opened)
	must(os.Rename(original, moved))
	must(os.Mkdir(original, 0o700))
	must(unix.Mkdirat(fd, "child", 0o700))
	if _, err := os.Stat(filepath.Join(moved, "child")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(original, "child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("handle rebound to old name", err)
	}
	raw, err := unix.Openat(opened, "data", unix.O_CREAT|unix.O_EXCL|unix.O_RDWR|unix.O_NOFOLLOW, 0o600)
	must(err)
	f := os.NewFile(uintptr(raw), "data")
	if f == nil {
		t.Fatal("NewFile returned nil")
	}
	defer f.Close()
	_, err = f.WriteString("inode-bound")
	must(err)
	_, err = f.Seek(0, io.SeekStart)
	must(err)
	data, err := io.ReadAll(f)
	must(err)
	if string(data) != "inode-bound" {
		t.Fatal(string(data))
	}
	must(f.Sync())
	must(dir.Sync())
	var stat unix.Stat_t
	must(unix.Fstatat(fd, "data", &stat, unix.AT_SYMLINK_NOFOLLOW))
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		t.Fatal(stat.Mode)
	}
	must(unix.Renameat(fd, "data", opened, "published"))
	must(unix.Unlinkat(fd, "published", 0))
	must(unix.Unlinkat(fd, "child", unix.AT_REMOVEDIR))
	must(dir.Close())
	if _, err := unix.Openat(fd, ".", unix.O_RDONLY, 0); !errors.Is(err, unix.EBADF) {
		t.Fatal("closed descriptor", err)
	}
	// Absolute lookup ignores the closed descriptor.
	abs, err := unix.Openat(fd, moved, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	must(err)
	must(unix.Close(abs))
	if _, err := unix.Openat(raw, "child", unix.O_RDONLY, 0); !errors.Is(err, unix.ENOTDIR) {
		t.Fatal("regular descriptor", err)
	}
}

func TestRootSymlinkConfinement(t *testing.T) {
	if err := os.MkdirAll(os.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	rootPath := filepath.Join(base, "root")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(rootPath, "sub"), 0o700))
	must(os.WriteFile(filepath.Join(rootPath, "sub", "file"), []byte("inside"), 0o600))
	must(os.WriteFile(filepath.Join(base, "outside"), []byte("outside"), 0o600))
	for name, target := range map[string]string{"inside": "sub/file", "dir": "sub", "escape": "../outside", "absolute": filepath.Join(base, "outside"), "dangling": "missing", "loop": "loop"} {
		must(os.Symlink(target, filepath.Join(rootPath, name)))
	}
	root, err := os.OpenRoot(rootPath)
	must(err)
	defer root.Close()
	for _, name := range []string{"sub/file", "inside", "dir/file", "sub/../inside"} {
		f, err := root.Open(name)
		must(err)
		data, err := io.ReadAll(f)
		must(err)
		must(f.Close())
		if string(data) != "inside" {
			t.Fatalf("%s: %q", name, data)
		}
	}
	info, err := root.Lstat("inside")
	must(err)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("Lstat followed link", info.Mode())
	}
	info, err = root.Stat("inside")
	must(err)
	if !info.Mode().IsRegular() {
		t.Fatal("Stat did not follow link", info.Mode())
	}
	for _, name := range []string{"../outside", "escape", "absolute", filepath.Join(base, "outside"), "loop", "dangling"} {
		f, err := root.Open(name)
		if err == nil {
			f.Close()
			t.Fatalf("Root accepted %q", name)
		}
	}
	if f, err := root.OpenFile("dangling", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); !errors.Is(err, os.ErrExist) {
		if f != nil {
			f.Close()
		}
		t.Fatal("exclusive create followed dangling symlink", err)
	}
	must(root.Mkdir("new", 0o700))
	f, err := root.Create("new/file")
	must(err)
	must(f.Close())
	must(root.Rename("new/file", "new/moved"))
	must(root.Remove("new/moved"))
	must(root.Remove("new"))
	renamed := filepath.Join(base, "renamed")
	must(os.Rename(rootPath, renamed))
	f, err = root.Open("inside")
	must(err)
	must(f.Close())
	must(root.Close())
	if f, err := root.Open("inside"); !errors.Is(err, os.ErrClosed) {
		if f != nil {
			f.Close()
		}
		t.Fatal(err)
	}
}

func TestDirectoryNoFollowSemantics(t *testing.T) {
	if err := os.MkdirAll(os.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(filepath.Join(base, "dir"), 0o700))
	must(os.WriteFile(filepath.Join(base, "dir", "file"), []byte("keep"), 0o600))
	must(os.Symlink("dir", filepath.Join(base, "link")))
	must(os.Symlink("dir/file", filepath.Join(base, "filelink")))
	d, err := os.Open(base)
	must(err)
	defer d.Close()
	fd := int(d.Fd())
	// NOFOLLOW applies only to the final component of a raw openat path.
	f, err := unix.Openat(fd, "link/file", unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	must(err)
	must(unix.Close(f))
	for _, tc := range []struct {
		name  string
		flags int
		want  error
	}{
		{"filelink", unix.O_RDONLY | unix.O_NOFOLLOW, unix.ELOOP},
		{"dir/file", unix.O_RDONLY | unix.O_DIRECTORY, unix.ENOTDIR},
		{"filelink", unix.O_CREAT | unix.O_EXCL | unix.O_WRONLY, unix.EEXIST},
	} {
		got, err := unix.Openat(fd, tc.name, tc.flags, 0o600)
		if !errors.Is(err, tc.want) {
			if err == nil {
				unix.Close(got)
			}
			t.Fatalf("%s flags %x: %v, want %v", tc.name, tc.flags, err, tc.want)
		}
	}
	var follow, nofollow unix.Stat_t
	must(unix.Fstatat(fd, "filelink", &follow, 0))
	must(unix.Fstatat(fd, "filelink", &nofollow, unix.AT_SYMLINK_NOFOLLOW))
	if follow.Mode&unix.S_IFMT != unix.S_IFREG || nofollow.Mode&unix.S_IFMT != unix.S_IFLNK {
		t.Fatal(follow.Mode, nofollow.Mode)
	}
	target := make([]byte, 3)
	n, err := unix.Readlinkat(fd, "filelink", target)
	must(err)
	if n != 3 || string(target) != "dir" {
		t.Fatal(n, string(target))
	}
	if err := unix.Unlinkat(fd, "filelink", unix.AT_REMOVEDIR); !errors.Is(err, unix.ENOTDIR) {
		t.Fatal(err)
	}
	if err := unix.Unlinkat(fd, "dir", unix.AT_REMOVEDIR); !errors.Is(err, unix.ENOTEMPTY) {
		t.Fatal(err)
	}
	must(unix.Unlinkat(fd, "filelink", 0))
	data, err := os.ReadFile(filepath.Join(base, "dir", "file"))
	must(err)
	if string(data) != "keep" {
		t.Fatal(string(data))
	}
}
