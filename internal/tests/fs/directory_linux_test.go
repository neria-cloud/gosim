package behavior_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// These Linux contracts are also required of simulation; unlike the native
// reference file, this file deliberately has no !sim constraint.
func TestGS01LinuxTerminalPolicies(t *testing.T) {
	base := descriptorTempDir(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	check(os.WriteFile(filepath.Join(base, "file"), []byte("keep"), 0o600))
	check(os.Mkdir(filepath.Join(base, "dir"), 0o700))
	check(os.Symlink("file", filepath.Join(base, "link")))
	dir, err := os.Open(base)
	check(err)
	defer dir.Close()
	fd := int(dir.Fd())
	for _, tc := range []struct {
		name  string
		flags int
		want  error
	}{
		{"file/", unix.O_CREAT, unix.EISDIR},
		{"dir/", unix.O_CREAT | unix.O_EXCL, unix.EISDIR},
		{"missing/", unix.O_CREAT, unix.EISDIR},
		{"link/", unix.O_CREAT, unix.EISDIR},
		{"link", unix.O_DIRECTORY | unix.O_NOFOLLOW, unix.ENOTDIR},
		{".", unix.O_CREAT | unix.O_EXCL, unix.EEXIST},
		{"..", unix.O_CREAT | unix.O_EXCL, unix.EEXIST},
	} {
		got, err := unix.Openat(fd, tc.name, tc.flags, 0o600)
		if !errors.Is(err, tc.want) {
			if err == nil {
				unix.Close(got)
			}
			t.Fatalf("%s flags=%x: %v want %v", tc.name, tc.flags, err, tc.want)
		}
	}
	for _, name := range []string{"file/", "dir/", "link/", ".", ".."} {
		if err := unix.Mkdirat(fd, name, 0o700); !errors.Is(err, unix.EEXIST) {
			t.Fatal(name, err)
		}
		if err := unix.Symlinkat("file", fd, name); !errors.Is(err, unix.EEXIST) {
			t.Fatal(name, err)
		}
	}
	if err := unix.Symlinkat("file", fd, "missing/"); !errors.Is(err, unix.ENOENT) {
		t.Fatal(err)
	}
	if _, err := unix.Openat(-999, "", unix.O_RDONLY, 0); !errors.Is(err, unix.ENOENT) {
		t.Fatal(err)
	}
	if err := unix.Symlinkat("", -999, "x"); !errors.Is(err, unix.ENOENT) {
		t.Fatal(err)
	}
	if err := unix.Symlinkat(strings.Repeat("x", 4096), -999, "x"); !errors.Is(err, unix.ENAMETOOLONG) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want error
	}{{".", unix.EINVAL}, {"..", unix.ENOTEMPTY}, {"/", unix.EBUSY}, {"file", unix.ENOTDIR}, {"link", unix.ENOTDIR}} {
		if err := unix.Unlinkat(fd, tc.name, unix.AT_REMOVEDIR); !errors.Is(err, tc.want) {
			t.Fatal(tc.name, err)
		}
	}
	opened, err := unix.Openat(fd, "file", unix.O_EXCL|unix.O_RDONLY, 0)
	check(err)
	check(unix.Close(opened))
	before, err := os.ReadFile(filepath.Join(base, "file"))
	check(err)
	if string(before) != "keep" {
		t.Fatal("failed opens changed contents", string(before))
	}
	opened, err = unix.Openat(fd, "file", unix.O_TRUNC|unix.O_RDONLY, 0)
	check(err)
	check(unix.Close(opened))
	info, err := os.Stat(filepath.Join(base, "file"))
	check(err)
	if info.Size() != 0 {
		t.Fatal("RDONLY|TRUNC did not truncate")
	}
}

func TestGS01LinuxRenamePolicies(t *testing.T) {
	base := descriptorTempDir(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"A/child", "empty", "full"} {
		check(os.MkdirAll(filepath.Join(base, name), 0o700))
	}
	check(os.WriteFile(filepath.Join(base, "file"), []byte("keep"), 0o600))
	check(os.WriteFile(filepath.Join(base, "full", "file"), nil, 0o600))
	dir, err := os.Open(base)
	check(err)
	defer dir.Close()
	fd := int(dir.Fd())
	for _, tc := range []struct {
		old, new string
		want     error
	}{
		{"A", "A/child/new", unix.EINVAL},
		{"A/child", "A", unix.ENOTEMPTY},
		{"full/file", "full", unix.ENOTEMPTY},
		{"file", "empty", unix.EISDIR},
		{"empty", "file", unix.ENOTDIR},
		{"A", "full", unix.ENOTEMPTY},
		{"file/", "other", unix.ENOTDIR},
		{"file", "other/", unix.ENOTDIR},
		{".", "other", unix.EBUSY},
		{"A", ".", unix.EBUSY},
	} {
		if err := unix.Renameat(fd, tc.old, fd, tc.new); !errors.Is(err, tc.want) {
			t.Fatalf("%s -> %s: %v want %v", tc.old, tc.new, err, tc.want)
		}
	}
	check(unix.Renameat(fd, "file", fd, "file"))
	check(unix.Renameat(fd, "A", fd, "empty"))
	if _, err := os.Stat(filepath.Join(base, "empty", "child")); err != nil {
		t.Fatal(err)
	}
}
