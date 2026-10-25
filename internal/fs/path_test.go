package fs

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestAtPathsChooseDescriptorOrAbsoluteRoot(t *testing.T) {
	i, ctx, table := ioFixture(t)
	held := open(t, i, ctx, ".", openDirectory)
	regular := open(t, i, ctx, "regular", openCreate|2)
	closed := open(t, i, ctx, ".", openDirectory)
	resource, err := table.Remove(closed)
	must(t, err)
	must(t, resource.Release(contracts.CloseExplicit))
	foreign, err := table.Allocate(wrongOwner{})
	must(t, err)
	must(t, i.Chdir(ctx, "/etc"))
	if _, err := i.Statat(ctx, held, "regular", 0); err != nil {
		t.Fatal("cwd redirected handle", err)
	}
	if _, err := i.Statat(ctx, atFDCWD, "regular", 0); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		fd   int
		want error
	}{
		{"bad", 9999, syscall.EBADF},
		{"closed", closed, syscall.EBADF},
		{"regular", regular, syscall.ENOTDIR},
		{"other-owner", foreign, syscall.ENOTDIR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, openErr := i.Openat(ctx, tc.fd, "regular", 0, 0)
			_, statErr := i.Statat(ctx, tc.fd, "regular", 0)
			for _, err := range []error{openErr, statErr, i.Mkdirat(ctx, tc.fd, "new", 0o700), i.Unlinkat(ctx, tc.fd, "regular", 0), i.Renameat(ctx, tc.fd, "regular", held, "other")} {
				if !errors.Is(err, tc.want) {
					t.Fatalf("relative descriptor: %v, want %v", err, tc.want)
				}
			}
			for _, err := range []error{i.Mkdirat(ctx, tc.fd, "", 0o700), i.Unlinkat(ctx, tc.fd, "", 0)} {
				if !errors.Is(err, syscall.ENOENT) {
					t.Fatal("empty path did not precede FD lookup", err)
				}
			}
			if _, err := i.Openat(ctx, tc.fd, "", 0, 0); !errors.Is(err, syscall.ENOENT) {
				t.Fatal(err)
			}
			if _, err := i.Statat(ctx, tc.fd, "", 0); !errors.Is(err, syscall.ENOENT) {
				t.Fatal(err)
			}
			path := "/app/" + tc.name
			fd, err := i.Openat(ctx, tc.fd, path, openCreate|2, 0o600)
			must(t, err)
			if _, err := i.Statat(ctx, tc.fd, path, atSymlinkNoFollow); err != nil {
				t.Fatal("absolute stat used FD", err)
			}
			must(t, i.Renameat(ctx, tc.fd, path, tc.fd, path+"-moved"))
			must(t, i.Unlinkat(ctx, tc.fd, path+"-moved", 0))
			if _, err := i.Fstat(ctx, fd); err != nil {
				t.Fatal("unlink invalidated open inode", err)
			}
			must(t, i.Mkdirat(ctx, tc.fd, path, 0o700))
			must(t, i.Unlinkat(ctx, tc.fd, path, atRemoveDir))
			// Each rename operand independently selects its base.
			open(t, i, ctx, path, openCreate|2)
			must(t, i.Renameat(ctx, tc.fd, path, held, tc.name+"-relative"))
			must(t, i.Renameat(ctx, held, tc.name+"-relative", tc.fd, path))
			must(t, i.Unlinkat(ctx, tc.fd, path, 0))
		})
	}
}

func TestHeldDirectoryIdentityAndReferenceRollback(t *testing.T) {
	i, ctx, table := ioFixture(t)
	must(t, i.Mkdirat(ctx, atFDCWD, "before", 0o700))
	fd := open(t, i, ctx, "before", openDirectory)
	before, err := i.Fstat(ctx, fd)
	must(t, err)
	must(t, i.Renameat(ctx, atFDCWD, "before", atFDCWD, "after"))
	must(t, i.Mkdirat(ctx, atFDCWD, "before", 0o700))
	must(t, i.Mkdirat(ctx, fd, "child", 0o700))
	if _, err := i.Statat(ctx, atFDCWD, "after/child", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Statat(ctx, atFDCWD, "before/child", 0); !errors.Is(err, syscall.ENOENT) {
		t.Fatal("rebound handle", err)
	}
	must(t, i.Sync(ctx, fd, false))
	i.fds = failingAllocation{table}
	if _, err := i.Openat(ctx, fd, ".", openDirectory, 0); !errors.Is(err, syscall.EMFILE) {
		t.Fatal(err)
	}
	i.fds = table
	if got := i.model.GetInodeInfo(int(before.Inode)).Handles; got != 1 {
		t.Fatal("allocation rollback lost or leaked reference", got)
	}
	stale := ctx
	stale.Epoch.Epoch++
	if _, err := i.Openat(stale, fd, ".", 0, 0); err == nil {
		t.Fatal("accepted stale epoch")
	}
	must(t, i.Unlinkat(ctx, fd, "child", atRemoveDir))
	must(t, i.Unlinkat(ctx, atFDCWD, "after", atRemoveDir))
	if _, err := i.Fstat(ctx, fd); err != nil {
		t.Fatal(err)
	}
	if err := i.Mkdirat(ctx, fd, "new", 0o700); !errors.Is(err, syscall.ENOENT) {
		t.Fatal("unlinked directory accepted entry", err)
	}
	resource, err := table.Remove(fd)
	must(t, err)
	must(t, resource.Release(contracts.CloseExplicit))
	must(t, resource.Release(contracts.CloseCrash))
	if got := i.model.GetInodeInfo(int(before.Inode)).Handles; got != 0 {
		t.Fatal(got)
	}
	if _, err := i.Openat(ctx, fd, ".", 0, 0); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
}

func TestDirectoryDescriptorRejectsAnotherInstance(t *testing.T) {
	i, ctx, table := ioFixture(t)
	other, otherCtx, otherTable := ioFixture(t)
	fd := open(t, other, otherCtx, ".", openDirectory)
	resource, err := otherTable.Lookup(fd)
	must(t, err)
	foreign, err := table.Allocate(resource)
	must(t, err)
	if _, err := i.Openat(ctx, foreign, ".", openDirectory, 0); !errors.Is(err, syscall.EBADF) {
		t.Fatal("cross-instance directory accepted", err)
	}
	// Absolute lookup does not inspect even this deliberately foreign resource.
	if _, err := i.Openat(ctx, foreign, "/app", openDirectory, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSymlinkProviderMetadataAndReadOnly(t *testing.T) {
	i, ctx, table := ioFixture(t)
	fd := open(t, i, ctx, "target", openCreate|2)
	resource, err := table.Lookup(fd)
	must(t, err)
	file := resource.(*File)
	_, err = file.Write(ctx, bytesView("keep"))
	must(t, err)
	must(t, i.Symlinkat(ctx, "target", atFDCWD, "link"))
	must(t, i.Chownat(ctx, atFDCWD, "link", 123, 456, atSymlinkNoFollow))
	link, err := i.Statat(ctx, atFDCWD, "link", atSymlinkNoFollow)
	must(t, err)
	target, err := i.Statat(ctx, atFDCWD, "link", 0)
	must(t, err)
	if link.Mode&0o170000 != 0o120000 || link.UID != 123 || target.UID != 0 || link.Inode == target.Inode {
		t.Fatal(link, target)
	}
	if err := i.Chmodat(ctx, atFDCWD, "link", 0, atSymlinkNoFollow); !errors.Is(err, syscall.EOPNOTSUPP) {
		t.Fatal(err)
	}
	must(t, i.Chmodat(ctx, atFDCWD, "link", 0o640, 0))
	stat, err := i.Fstat(ctx, fd)
	must(t, err)
	if stat.Mode&0o777 != 0o640 {
		t.Fatal(stat)
	}
	buf := []byte("????")
	n, err := i.Readlinkat(ctx, atFDCWD, "link", abi.ByteView{Ptr: buf[:3]})
	must(t, err)
	if n != 3 || string(buf) != "tar?" {
		t.Fatal(n, string(buf))
	}
	if _, err := i.Readlinkat(ctx, 999, "link", abi.ByteView{}); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		target string
		want   error
	}{{"", syscall.ENOENT}, {"a\x00b", syscall.EINVAL}, {strings.Repeat("a", 4096), syscall.ENAMETOOLONG}} {
		if err := i.Symlinkat(ctx, tc.target, 999, "new"); !errors.Is(err, tc.want) {
			t.Fatal("target validation order", err, tc.want)
		}
	}
	must(t, i.session.SetReadOnly(ref1, true))
	for name, op := range map[string]func() error{
		"symlink": func() error { return i.Symlinkat(ctx, "target", atFDCWD, "new") },
		"mkdir":   func() error { return i.Mkdirat(ctx, atFDCWD, "new", 0o700) },
		"rename":  func() error { return i.Renameat(ctx, atFDCWD, "link", atFDCWD, "new") },
		"unlink":  func() error { return i.Unlinkat(ctx, atFDCWD, "link", 0) },
		"create":  func() error { _, err := i.Openat(ctx, atFDCWD, "new", openCreate, 0o600); return err },
		"write":   func() error { _, err := file.Write(ctx, bytesView("bad")); return err },
	} {
		if err := op(); !errors.Is(err, syscall.EROFS) {
			t.Fatal(name, err)
		}
	}
	if file.pos != 4 {
		t.Fatal("read-only write advanced position", file.pos)
	}
	if _, err := i.Statat(ctx, atFDCWD, "new", 0); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	if _, err := i.Openat(ctx, atFDCWD, "target", 0, 0); err != nil {
		t.Fatal("read-only open failed", err)
	}
	must(t, i.session.SetReadOnly(ref1, false))
	must(t, i.Symlinkat(ctx, "target", atFDCWD, "new"))
	for _, dirfd := range []int{atFDCWD, fd} {
		if _, err := i.Openat(ctx, dirfd, "/app/target", openDirectory|openTruncate, 0); !errors.Is(err, syscall.ENOTDIR) {
			t.Fatal(err)
		}
	}
	buf = make([]byte, 4)
	_, err = i.Pread(ctx, fd, abi.ByteView{Ptr: buf}, 0)
	must(t, err)
	if string(buf) != "keep" {
		t.Fatal(fmt.Sprintf("failed opens changed target: %q", buf))
	}
}
