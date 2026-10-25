package behavior_test

import (
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/unix"
)

// These cases exercise the same public Go/x/sys entry points natively and in
// simulation. Linux-specific errno/flag precedence has separate coverage.
func TestDirectoryDescriptorRenameAndClose(t *testing.T) {
	base := descriptorTempDir(t)
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
	previous, err := os.Getwd()
	must(err)
	must(os.Chdir(base))
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	}()
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
	base := descriptorTempDir(t)
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
	base := descriptorTempDir(t)
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

func descriptorTempDir(t *testing.T) string {
	t.Helper()
	setupRealDisk(t)
	dir, err := os.MkdirTemp(".", "gs01-")
	if err != nil {
		t.Fatal(err)
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return dir
}

func TestRootMethodsAndFilesystemView(t *testing.T) {
	base := descriptorTempDir(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(base)
	check(err)
	defer root.Close()
	if root.Name() != base {
		t.Fatal(root.Name(), base)
	}
	check(root.MkdirAll("one/two", 0o750))
	check(root.WriteFile("one/two/file", []byte("inside"), 0o600))
	check(root.Symlink("two/file", "one/link"))
	target, err := root.Readlink("one/link")
	check(err)
	if target != "two/file" {
		t.Fatal(target)
	}
	check(root.Chmod("one/link", 0o640))
	check(root.Chown("one/two/file", -1, -1))
	check(root.Lchown("one/link", -1, -1))
	info, err := root.Stat("one/link")
	check(err)
	if info.Mode().Perm() != 0o640 || !info.Mode().IsRegular() {
		t.Fatal(info.Mode())
	}
	view := root.FS()
	opened, err := view.Open("one/two/file")
	check(err)
	check(opened.Close())
	viewed, err := stdfs.Stat(view, "one/link")
	check(err)
	if !viewed.Mode().IsRegular() {
		t.Fatal(viewed.Mode())
	}
	data, err := stdfs.ReadFile(view, "one/link")
	check(err)
	if string(data) != "inside" {
		t.Fatal(string(data))
	}
	entries, err := stdfs.ReadDir(view, "one")
	check(err)
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{"link", "two"}) {
		t.Fatal(names)
	}
	readlinks, ok := view.(interface{ ReadLink(string) (string, error) })
	if !ok {
		t.Fatal("Root.FS lacks ReadLink")
	}
	target, err = readlinks.ReadLink("one/link")
	check(err)
	if target != "two/file" {
		t.Fatal(target)
	}
	statfs, ok := view.(interface {
		Lstat(string) (os.FileInfo, error)
	})
	if !ok {
		t.Fatal("Root.FS lacks Lstat")
	}
	info, err = statfs.Lstat("one/link")
	check(err)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal(info.Mode())
	}
	if _, err := view.Open("../outside"); err == nil {
		t.Fatal("FS accepted invalid path")
	}
	nested, err := root.OpenRoot("one")
	check(err)
	defer nested.Close()
	data, err = nested.ReadFile("link")
	check(err)
	if string(data) != "inside" {
		t.Fatal(string(data))
	}
	outside := filepath.Join(base, "outside")
	check(root.WriteFile("outside", []byte("keep"), 0o600))
	check(root.Symlink(outside, "one/two/escape"))
	check(root.RemoveAll("one"))
	data, err = os.ReadFile(outside)
	check(err)
	if string(data) != "keep" {
		t.Fatal("RemoveAll followed link outside removed tree")
	}
}

func TestDirectoryRawTraversalIsDistinctFromRootConfinement(t *testing.T) {
	base := descriptorTempDir(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	check(os.Mkdir(filepath.Join(base, "root"), 0o700))
	check(os.WriteFile(filepath.Join(base, "outside"), []byte("outside"), 0o600))
	check(os.Symlink("..", filepath.Join(base, "root", "up")))
	directory, err := os.Open(filepath.Join(base, "root"))
	check(err)
	defer directory.Close()
	fd, err := unix.Openat(int(directory.Fd()), "up/outside", unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	check(err)
	file := os.NewFile(uintptr(fd), "outside")
	defer file.Close()
	data, err := io.ReadAll(file)
	check(err)
	if string(data) != "outside" {
		t.Fatal(string(data))
	}
	root, err := os.OpenRoot(filepath.Join(base, "root"))
	check(err)
	defer root.Close()
	if f, err := root.Open("up/outside"); err == nil {
		f.Close()
		t.Fatal("Root escaped through intermediate link")
	}
}

func TestDirectoryRewindAndHeldReplacement(t *testing.T) {
	base := descriptorTempDir(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"old", "replacement"} {
		check(os.Mkdir(filepath.Join(base, name), 0o700))
	}
	check(os.WriteFile(filepath.Join(base, "old", "a"), nil, 0o600))
	dir, err := os.Open(filepath.Join(base, "old"))
	check(err)
	defer dir.Close()
	first, err := dir.Readdirnames(-1)
	check(err)
	_, err = dir.Seek(0, io.SeekStart)
	check(err)
	second, err := dir.Readdirnames(-1)
	check(err)
	if !slices.Equal(first, second) || len(second) != 1 {
		t.Fatal(first, second)
	}
	replaced, err := os.Open(filepath.Join(base, "replacement"))
	check(err)
	defer replaced.Close()
	before, err := replaced.Stat()
	check(err)
	check(unix.Renameat(unix.AT_FDCWD, filepath.Join(base, "old"), unix.AT_FDCWD, filepath.Join(base, "replacement")))
	after, err := replaced.Stat()
	check(err)
	if !os.SameFile(before, after) {
		t.Fatal("overwritten directory handle rebound")
	}
	held, err := dir.Stat()
	check(err)
	named, err := os.Stat(filepath.Join(base, "replacement"))
	check(err)
	if !os.SameFile(held, named) {
		t.Fatal("moved directory lost identity")
	}
	check(dir.Sync())
	check(replaced.Sync())
}

func TestDirectoryAbsoluteOperandsIgnoreInvalidFD(t *testing.T) {
	base := descriptorTempDir(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	const invalid = -999
	folder := filepath.Join(base, "folder")
	check(unix.Mkdirat(invalid, folder, 0o700))
	old := filepath.Join(folder, "old")
	fd, err := unix.Openat(invalid, old, unix.O_CREAT|unix.O_RDWR|unix.O_EXCL, 0o600)
	check(err)
	check(unix.Close(fd))
	var info unix.Stat_t
	check(unix.Fstatat(invalid, old, &info, unix.AT_SYMLINK_NOFOLLOW))
	moved := filepath.Join(folder, "moved")
	check(unix.Renameat(invalid, old, invalid, moved))
	dir, err := os.Open(folder)
	check(err)
	defer dir.Close()
	check(unix.Renameat(invalid, moved, int(dir.Fd()), "relative"))
	check(unix.Renameat(int(dir.Fd()), "relative", invalid, moved))
	link := filepath.Join(folder, "link")
	check(unix.Symlinkat("moved", invalid, link))
	target := make([]byte, 32)
	n, err := unix.Readlinkat(invalid, link, target)
	check(err)
	if string(target[:n]) != "moved" {
		t.Fatal("absolute readlink used directory descriptor", string(target[:n]))
	}
	check(unix.Fchmodat(invalid, moved, 0o640, 0))
	check(unix.Fchownat(invalid, link, -1, -1, unix.AT_SYMLINK_NOFOLLOW))
	check(unix.Fstatat(invalid, moved, &info, 0))
	if info.Mode&0o777 != 0o640 {
		t.Fatal("absolute chmod did not reach file", info.Mode)
	}
	check(unix.Unlinkat(invalid, link, 0))
	check(unix.Unlinkat(invalid, moved, 0))
	check(unix.Unlinkat(invalid, folder, unix.AT_REMOVEDIR))
}

func TestRootDirectoryDescriptorChain(t *testing.T) {
	base := descriptorTempDir(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(base)
	check(err)
	defer root.Close()
	check(root.Mkdir("child", 0o700))
	parent, err := root.Open(".")
	check(err)
	defer parent.Close()
	fd, err := unix.Openat(int(parent.Fd()), "child", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	check(err)
	child := os.NewFile(uintptr(fd), "child")
	if child == nil {
		unix.Close(fd)
		t.Fatal("NewFile rejected directory descriptor")
	}
	defer child.Close()
	held, err := child.Stat()
	check(err)
	named, err := root.Stat("child")
	check(err)
	if !held.IsDir() || !os.SameFile(held, named) {
		t.Fatal("directory descriptor changed inode")
	}
	check(root.Rename("child", "renamed"))
	named, err = root.Stat("renamed")
	check(err)
	if !os.SameFile(held, named) {
		t.Fatal("rename changed directory identity")
	}
	// Closing both ancestors must not release the independently opened child.
	check(parent.Close())
	check(root.Close())
	check(root.Close())
	check(child.Sync())
	check(child.Close())
	if opened, err := unix.Openat(fd, ".", unix.O_RDONLY|unix.O_DIRECTORY, 0); !errors.Is(err, unix.EBADF) {
		if err == nil {
			unix.Close(opened)
		}
		t.Fatal("closed directory descriptor remained usable", err)
	}
	if opened, err := root.Open("."); !errors.Is(err, os.ErrClosed) {
		if opened != nil {
			opened.Close()
		}
		t.Fatal("closed Root remained usable", err)
	}
}
