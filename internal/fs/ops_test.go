package fs

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"slices"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/fs/model"
	"github.com/neria-cloud/gosim/internal/kernel"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

const (
	fileOpsMode             = 0o600
	fileOpsDirMode          = 0o700
	fileOpsInputSize        = 6
	fileOpsSeekBack         = 3
	fileOpsAllocation       = 10
	fileOpsTruncatedSize    = 8
	fileOpsCwdBufferSize    = 64
	fileOpsDirentSize       = 25
	fileOpsDirentType       = 8
	fileOpsLockCommand      = 123
	fileOpsAdvisoryValue    = 123
	fileOpsLockType         = 7
	fileOpsLockPID          = 99
	fileOpsUnknownStatFlag  = 0x1000
	fileOpsInvalidOpenMode  = 3
	fileOpsFileTypeMask     = 0o170000
	fileOpsRegularFileType  = 0o100000
	fileOpsPermissionMask   = 0o777
	fileOpsDirentTypeOffset = 18
)

func ioFixture(t *testing.T) (*Instance, contracts.CallContext, *kernel.FDTable) {
	t.Helper()
	s, _ := newSession(t)
	epoch := contracts.EpochRef{Machine: ref1, Epoch: 1}
	table := kernel.NewFDTable(epoch)
	p, err := s.Prepare(contracts.StartSpec{Ref: epoch}, table)
	must(t, err)
	i := p.Publish().(*Instance)
	t.Cleanup(func() { must(t, table.Finalize(contracts.CloseCrash)); i.Detach() })
	return i, contracts.CallContext{Epoch: epoch}, table
}

func open(t *testing.T, i *Instance, ctx contracts.CallContext, path string, flags int) int {
	t.Helper()
	fd, err := i.Openat(ctx, atFDCWD, path, flags, fileOpsMode)
	must(t, err)
	return fd
}
func bytesView(value string) abi.ByteView { return abi.ByteView{Ptr: []byte(value)} }
func TestFileOpsOffsetsDurabilityAndBorrowing(t *testing.T) {
	t.Parallel()
	i, ctx, table := ioFixture(t)
	fd := open(t, i, ctx, "data", openCreate|2|openCloseExec)
	resource, err := table.Lookup(fd)
	must(t, err)
	file := resource.(*File)
	input := bytesView("abcdef")
	n, err := file.Write(ctx, input)
	must(t, err)
	if n != fileOpsInputSize {
		t.Fatal(n)
	}
	input.Ptr[0] = 'X'
	pos, err := i.Seek(ctx, fd, -fileOpsSeekBack, io.SeekCurrent)
	must(t, err)
	if pos != fileOpsSeekBack {
		t.Fatal(pos)
	}
	output := make([]byte, fileOpsSeekBack)
	n, err = file.Read(ctx, abi.ByteView{Ptr: output})
	must(t, err)
	if n != fileOpsSeekBack || string(output) != "def" {
		t.Fatal(string(output))
	}
	_, err = i.Pwrite(ctx, fd, bytesView("Q"), 1)
	must(t, err)
	if file.pos != fileOpsInputSize {
		t.Fatal("pwrite changed offset")
	}
	all := make([]byte, fileOpsInputSize)
	_, err = i.Pread(ctx, fd, abi.ByteView{Ptr: all}, 0)
	must(t, err)
	if string(all) != "aQcdef" || file.pos != fileOpsInputSize {
		t.Fatal("borrowed write buffer or positional offset", string(all))
	}
	must(t, i.Fallocate(ctx, fd, 0, 0, fileOpsAllocation))
	must(t, i.Truncate(ctx, fd, fileOpsTruncatedSize))
	stat, err := i.Fstat(ctx, fd)
	must(t, err)
	if stat.Size != fileOpsTruncatedSize || stat.Mode&fileOpsFileTypeMask != fileOpsRegularFileType ||
		stat.Mode&fileOpsPermissionMask == 0 || stat.UID != 0 || stat.Nlink != 1 || stat.AtimeNS != 0 {
		t.Fatal("invented stat metadata", stat)
	}
	must(t, i.Sync(ctx, fd, true))
	info, err := i.session.InspectInode(ref1, file.inode)
	must(t, err)
	if !info.DiskExists || info.DiskLinks != 0 || info.Ops != 1 {
		t.Fatal("sync did not persist inode", info)
	}
	if _, err := i.Seek(ctx, fd, math.MaxInt64, io.SeekCurrent); !errors.Is(err, syscall.EINVAL) {
		t.Fatal("overflowing seek accepted", err)
	}
	if _, err := i.Pread(ctx, fd, abi.ByteView{Ptr: all}, -1); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
	if _, err := i.Pwrite(ctx, fd, abi.ByteView{Ptr: all}, math.MaxInt64); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
	if err := i.Truncate(ctx, fd, -1); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
}

func TestFileOpsDirectoriesAndCapabilities(t *testing.T) {
	t.Parallel()
	i, ctx, _ := ioFixture(t)
	must(t, i.Mkdirat(ctx, atFDCWD, "sub", fileOpsDirMode))
	if err := i.Mkdirat(ctx, atFDCWD, "sub", fileOpsDirMode); !errors.Is(err, syscall.EEXIST) {
		t.Fatal(err)
	}
	must(t, i.Chdir(ctx, "sub"))
	buf := make([]byte, fileOpsCwdBufferSize)
	n, err := i.Getcwd(ctx, abi.ByteView{Ptr: buf})
	must(t, err)
	if string(buf[:n]) != "/app/sub\x00" {
		t.Fatal(string(buf[:n]))
	}
	fd := open(t, i, ctx, "before", openCreate|2)
	if _, err := i.Openat(ctx, atFDCWD, "before", openCreate|openExclusive|2, 0); !errors.Is(err, syscall.EEXIST) {
		t.Fatal(err)
	}
	must(t, i.Renameat(ctx, atFDCWD, "before", atFDCWD, "after"))
	stat, err := i.Statat(ctx, atFDCWD, "after", 0)
	must(t, err)
	if stat.Size != 0 {
		t.Fatal(stat)
	}
	dir := open(t, i, ctx, ".", openDirectory)
	n, err = i.Getdents64(ctx, dir, abi.ByteView{Ptr: buf})
	must(t, err)
	if n != fileOpsDirentSize || binary.LittleEndian.Uint16(buf[16:18]) != fileOpsDirentSize ||
		buf[fileOpsDirentTypeOffset] != fileOpsDirentType || string(buf[19:25]) != "after\x00" {
		t.Fatal("dirent encoding changed", buf[:n])
	}
	n, err = i.Getdents64(ctx, dir, abi.ByteView{Ptr: buf})
	must(t, err)
	if n != 0 {
		t.Fatal("historical one-batch behavior changed")
	}
	must(t, i.Flock(ctx, fd, fileOpsLockCommand))
	lock := contracts.Lock{Type: fileOpsLockType, PID: fileOpsLockPID}
	must(t, i.FcntlLock(ctx, fd, fileOpsLockCommand, &lock))
	if lock.PID != fileOpsLockPID {
		t.Fatal("lock stub changed input")
	}
	if _, err := i.FcntlInt(ctx, fd, 1, 0); !errors.Is(err, syscall.ENOSYS) {
		t.Fatal(err)
	}
	must(t, i.Advise(ctx, abi.ByteView{Ptr: buf}, fileOpsAdvisoryValue))
	must(t, i.Unlinkat(ctx, atFDCWD, "after", 0))
	if _, err := i.Statat(ctx, atFDCWD, "after", 0); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	if _, err := i.Fstat(ctx, fd); err != nil {
		t.Fatal("open unlinked file lost inode", err)
	}
	names := make(map[string]bool)
	for _, capability := range Capabilities() {
		if names[capability.Operation] {
			t.Fatal("duplicate capability")
		}
		names[capability.Operation] = true
	}
	for _, name := range []string{
		"openat", "close", "read", "write", "pread64", "pwrite64", "lseek", "fdatasync", "fsync",
		"fallocate", "ftruncate", "flock", "fstat", "newfstatat", "getdents64", "mkdirat",
		"fchmod", "fchmodat", "renameat", "unlinkat", "fcntl", "readlinkat", "statfs",
		"mmap", "munmap", "madvise",
	} {
		if !names[name] {
			t.Fatal("unclassified report operation", name)
		}
	}
}

func TestStatatNoFollow(t *testing.T) {
	t.Parallel()
	i, ctx, _ := ioFixture(t)
	open(t, i, ctx, "file", openCreate|2)
	for _, path := range []string{"/app", "/app/file"} {
		stat, err := i.Statat(ctx, atFDCWD, path, 0)
		must(t, err)
		lstat, err := i.Statat(ctx, atFDCWD, path, atSymlinkNoFollow)
		must(t, err)
		if lstat != stat {
			t.Fatalf("%s: stat=%+v lstat=%+v", path, stat, lstat)
		}
	}
	if _, err := i.Statat(ctx, atFDCWD, "/app/missing", atSymlinkNoFollow); !errors.Is(err, syscall.ENOENT) {
		t.Fatal(err)
	}
	if _, err := i.Statat(ctx, atFDCWD, "/app", fileOpsUnknownStatFlag); !errors.Is(err, syscall.EINVAL) {
		t.Fatal("unmodeled flags accepted", err)
	}
}

type wrongOwner struct{}

func (wrongOwner) Owner() contracts.Feature            { return contracts.Net }
func (wrongOwner) Release(contracts.CloseReason) error { return nil }

type failingAllocation struct{ contracts.DescriptorTable }

func (failingAllocation) Allocate(contracts.Resource) (int, error) { return 0, syscall.EMFILE }
func TestFileDescriptorOwnerFailuresAndCleanup(t *testing.T) {
	t.Parallel()
	i, ctx, table := ioFixture(t)
	foreign, err := table.Allocate(wrongOwner{})
	must(t, err)
	if _, err := i.Pread(ctx, foreign, bytesView("?"), 0); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
	i.fds = failingAllocation{table}
	if _, err := i.Openat(ctx, atFDCWD, "allocation", openCreate|2, 0); !errors.Is(err, syscall.EMFILE) {
		t.Fatal(err)
	}
	stat, err := i.model.Stat(4, "allocation", model.Identity{})
	must(t, err)
	if i.model.GetInodeInfo(stat.Inode).Handles != 0 {
		t.Fatal("failed allocation leaked inode reference")
	}
	i.fds = table
	fd := open(t, i, ctx, "allocation", 0)
	resource, err := table.Remove(fd)
	must(t, err)
	must(t, resource.Release(contracts.CloseExplicit))
	must(t, resource.Release(contracts.CloseCrash))
	file := resource.(*File)
	if file.closeReason != contracts.CloseExplicit || i.model.GetInodeInfo(file.inode).Handles != 0 {
		t.Fatal("release was not exactly once")
	}
	if _, err := file.Read(ctx, bytesView("?")); !errors.Is(err, syscall.EBADF) {
		t.Fatal("closed file readable", err)
	}
	for _, flags := range []int{fileOpsInvalidOpenMode, 0x400, 0x800, 0x20000000} {
		if _, err := i.Openat(ctx, atFDCWD, "unsupported", flags, 0); !errors.Is(err, syscall.EINVAL) {
			t.Fatal(flags, err)
		}
	}
	if _, err := i.Openat(ctx, foreign, "unsupported", 0, 0); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
	ro := open(t, i, ctx, "allocation", 0)
	if _, err := i.Pwrite(ctx, ro, bytesView("!"), 0); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
	if err := i.Truncate(ctx, ro, 1); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
	directory := open(t, i, ctx, ".", 0)
	if _, err := i.Pread(ctx, directory, bytesView("?"), 0); !errors.Is(err, syscall.EISDIR) {
		t.Fatal(err)
	}
}

func TestMappingSurvivesUnlinkCloseAndCleansUp(t *testing.T) {
	t.Parallel()
	i, ctx, table := ioFixture(t)
	fd := open(t, i, ctx, "mapped", openCreate|2)
	_, err := i.Pwrite(ctx, fd, bytesView("data"), 0)
	must(t, err)
	must(t, i.Sync(ctx, fd, false))
	directory := open(t, i, ctx, ".", 0)
	must(t, i.Sync(ctx, directory, false))
	address, err := i.Map(ctx, 0, 8, protRead, mapShared|mapPopulate, fd, 0)
	must(t, err)
	mapping := i.mappings[address]
	if !slices.Equal(mapping.Data.Ptr, []byte{'d', 'a', 't', 'a', 0, 0, 0, 0}) {
		t.Fatal(mapping.Data.Ptr)
	}
	_, err = i.Pwrite(ctx, fd, bytesView("X"), 1)
	must(t, err)
	if string(mapping.Data.Ptr[:4]) != "dXta" {
		t.Fatal("mapping did not observe write")
	}
	must(t, i.Sync(ctx, fd, false))
	must(t, i.Unlinkat(ctx, atFDCWD, "mapped", 0))
	must(t, i.Sync(ctx, directory, false))
	file, err := table.Remove(fd)
	must(t, err)
	must(t, file.Release(contracts.CloseExplicit))
	if !i.model.GetInodeInfo(mapping.Inode).MemExists {
		t.Fatal("close freed mapped inode")
	}
	must(t, i.Unmap(ctx, address, 1)) // historical unmap ignores length
	if i.model.GetInodeInfo(mapping.Inode).MemExists {
		t.Fatal("unmap retained unlinked inode")
	}
	if err := i.Unmap(ctx, address, 8); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
	fd = open(t, i, ctx, "other", openCreate|2)
	for _, length := range []uintptr{0, maxMapping + 1} {
		if _, err := i.Map(ctx, 0, length, protRead, mapShared, fd, 0); !errors.Is(err, syscall.EINVAL) {
			t.Fatal(err)
		}
	}
	if _, err := i.Map(ctx, 0, 8, 3, mapShared, fd, 0); !errors.Is(err, syscall.EINVAL) {
		t.Fatal("writable mapping accepted")
	}
	if _, err := i.Map(ctx, 0, 8, protRead, mapShared, fd, 1); !errors.Is(err, syscall.EINVAL) {
		t.Fatal("offset mapping accepted")
	}
	_, err = i.Map(ctx, 0, 8, protRead, mapShared, fd, 0)
	must(t, err)
	must(t, table.Finalize(contracts.CloseCrash))
	i.Detach()
	if len(i.mappings) != 0 || len(i.mappingOrder) != 0 {
		t.Fatal("epoch retained mappings")
	}
}

func TestStatErrorPreservesErrno(t *testing.T) {
	t.Parallel()
	for _, err := range []error{nil, syscall.ENOENT, syscall.ENOTDIR, syscall.EACCES} {
		if got := statError(err); got != err {
			t.Fatalf("statError(%v)=%v", err, got)
		}
	}
	if got := statError(errors.New("unclassified model error")); got != syscall.EINVAL {
		t.Fatal(got)
	}
}
