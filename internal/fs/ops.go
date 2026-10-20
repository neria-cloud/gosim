package fs

import (
	"encoding/binary"
	"io"
	"math"
	"os"
	"slices"
	"syscall"
	"unsafe"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/fs/model"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// Arguments describe simulated Linux, independently of the host running tests.
const (
	atFDCWD           = -100
	atRemoveDir       = 0x200
	atSymlinkNoFollow = 0x100
	openCreate        = 0x40
	openExclusive     = 0x80
	openTruncate      = 0x200
	openCloseExec     = 0x80000
	mapShared         = 1
	mapPopulate       = 0x8000
	protRead          = 1
	maxMapping        = 16 * 1024 * 1024
)

func (i *Instance) dirInode(dirfd int) (int, error) {
	// TODO(not planned): directory-relative descriptors beyond AT_FDCWD.
	if dirfd != atFDCWD {
		return 0, syscall.EINVAL
	}
	return i.workingDir, nil
}

func (i *Instance) Openat(ctx contracts.CallContext, dirfd int, path string, flags int, mode uint32) (int, error) {
	if err := i.validateCall(ctx); err != nil {
		return 0, err
	}
	dir, err := i.dirInode(dirfd)
	if err != nil {
		return 0, err
	}
	flags &^= openCloseExec
	const supported = 3 | openCreate | openExclusive | openTruncate | openDirectory
	if flags&^supported != 0 || flags&3 == 3 {
		return 0, syscall.EINVAL
	}
	if i.machine.readOnly && flags&(openCreate|openTruncate) != 0 {
		return 0, syscall.EROFS
	}
	// TODO(not planned): permissions, O_APPEND, and strict O_DIRECTORY semantics.
	modelFlags := flags & 3
	for _, flag := range []struct{ linux, host int }{{openCreate, os.O_CREATE}, {openExclusive, os.O_EXCL}, {openTruncate, os.O_TRUNC}} {
		if flags&flag.linux != 0 {
			modelFlags |= flag.host
		}
	}
	inode, err := i.model.OpenFile(dir, path, modelFlags)
	if err != nil {
		return 0, err
	}
	f := &File{instance: i, inode: inode, read: flags&3 != 1, write: flags&3 != 0}
	fd, err := i.fds.Allocate(f)
	if err != nil {
		_ = f.Release(contracts.CloseAbort)
		return 0, err
	}
	return fd, nil
}

func (i *Instance) Renameat(ctx contracts.CallContext, oldFD int, oldPath string, newFD int, newPath string) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	old, err := i.dirInode(oldFD)
	if err != nil {
		return err
	}
	new, err := i.dirInode(newFD)
	if err != nil {
		return err
	}
	err = i.model.Rename(old, oldPath, new, newPath)
	if err != nil && err != syscall.ENOENT {
		return syscall.EINVAL
	}
	return err
}

func (i *Instance) Getdents64(ctx contracts.CallContext, fd int, data abi.ByteView) (int, error) {
	f, err := i.file(ctx, fd)
	if err != nil {
		return 0, err
	}
	if f.didReaddir {
		return 0, nil
	}
	entries, err := i.model.ReadDir(f.inode)
	if err != nil {
		return 0, syscall.EINVAL
	}
	// This explicit Linux dirent64 byte layout is shared by both target arches.
	// TODO(not planned): resumable directory cursor and real inode/offset fields.
	n := 0
	for _, entry := range entries {
		length := 19 + len(entry.Name) + 1
		if length > 4096 {
			return 0, syscall.EINVAL
		}
		if length > data.Len()-n {
			break
		}
		record := make([]byte, length)
		binary.LittleEndian.PutUint64(record[0:8], 1)
		binary.LittleEndian.PutUint64(record[8:16], 1)
		binary.LittleEndian.PutUint16(record[16:18], uint16(length))
		record[18] = 8 // DT_REG
		if entry.IsDir {
			record[18] = 4
		}
		copy(record[19:], entry.Name)
		data.Slice(n, n+length).Write(record)
		n += length
	}
	f.didReaddir = true
	return n, nil
}

func (i *Instance) Fallocate(ctx contracts.CallContext, fd int, mode uint32, offset, length int64) error {
	f, err := i.file(ctx, fd)
	if err != nil {
		return err
	}
	if mode != 0 || offset != 0 || length < 0 {
		return syscall.EINVAL
	}
	if !f.write {
		return syscall.EBADF
	}
	stat, err := i.model.Statfd(f.inode)
	if err != nil {
		return err
	}
	if stat.IsDir {
		return syscall.EISDIR
	}
	if i.machine.readOnly {
		return syscall.EROFS
	}
	if length > stat.Size {
		if err := i.model.Truncate(f.inode, length, i.machine.config.CapacityBytes); err != nil {
			return err
		}
	}
	return nil
}

func (i *Instance) Seek(ctx contracts.CallContext, fd int, offset int64, whence int) (int64, error) {
	f, err := i.file(ctx, fd)
	if err != nil {
		return 0, err
	}
	var base int64
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = f.pos
	case io.SeekEnd:
		stat, err := i.model.Statfd(f.inode)
		if err != nil {
			return 0, err
		}
		base = stat.Size
	default:
		return 0, syscall.EINVAL
	}
	if offset > 0 && base > math.MaxInt64-offset || offset < -base {
		return 0, syscall.EINVAL
	}
	pos := base + offset
	if pos < 0 {
		return 0, syscall.EINVAL
	}
	f.pos = pos
	return pos, nil
}

func (i *Instance) Pwrite(ctx contracts.CallContext, fd int, data abi.ByteView, offset int64) (int, error) {
	f, err := i.file(ctx, fd)
	if err != nil {
		return 0, err
	}
	return f.writeAt(ctx, data, offset)
}

func (i *Instance) Pread(ctx contracts.CallContext, fd int, data abi.ByteView, offset int64) (int, error) {
	f, err := i.file(ctx, fd)
	if err != nil {
		return 0, err
	}
	return f.readAt(ctx, data, offset)
}

func (i *Instance) Sync(ctx contracts.CallContext, fd int, dataOnly bool) error {
	f, err := i.file(ctx, fd)
	if err != nil {
		return err
	}
	// fdatasync deliberately retains the same durability boundary as fsync.
	fault := i.machine.fault
	crash, before := false, false
	if fault.Probability > 0 {
		random := i.session.ports.RandomFor(contracts.FS, i.ref.Machine)
		crash = random.Float64() < fault.Probability
		if crash {
			before = random.Float64() < fault.BeforeProbability
		}
	}
	if !crash || !before {
		i.model.Sync(f.inode)
	}
	if crash {
		phase := "after"
		if before {
			phase = "before"
		}
		i.session.ports.Trace.Emit(i.ref, "sync-fault", []contracts.Field{{Key: "phase", Value: phase}})
		return &contracts.CrashRequest{Epoch: i.ref, Operation: "sync"}
	}
	return nil
}

func statResult(stat model.StatResp) contracts.Stat {
	mode := uint32(0o100000)
	if stat.IsDir {
		mode = 0o040000
	}
	return contracts.Stat{Inode: uint64(stat.Inode), Size: stat.Size, Mode: mode}
}

func statError(err error) error {
	if err == nil {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno
	}
	return syscall.EINVAL
}

func (i *Instance) Fstat(ctx contracts.CallContext, fd int) (contracts.Stat, error) {
	f, err := i.file(ctx, fd)
	if err != nil {
		return contracts.Stat{}, err
	}
	stat, err := i.model.Statfd(f.inode)
	return statResult(stat), statError(err)
}

func (i *Instance) Statat(ctx contracts.CallContext, dirfd int, path string, flags int) (contracts.Stat, error) {
	if err := i.validateCall(ctx); err != nil {
		return contracts.Stat{}, err
	}
	dir, err := i.dirInode(dirfd)
	if err != nil {
		return contracts.Stat{}, err
	}
	// The model contains files and directories only, so following and not
	// following the final component have identical results. Accept lstat's
	// Linux flag without implying support for symlink creation.
	if flags&^atSymlinkNoFollow != 0 {
		return contracts.Stat{}, syscall.EINVAL
	}
	stat, err := i.model.Stat(dir, path)
	return statResult(stat), statError(err)
}

func (i *Instance) Unlinkat(ctx contracts.CallContext, dirfd int, path string, flags int) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	dir, err := i.dirInode(dirfd)
	if err != nil {
		return err
	}
	if flags != 0 && flags != atRemoveDir {
		return syscall.EINVAL
	}
	return i.model.Remove(dir, path, flags == atRemoveDir)
}

func (i *Instance) Truncate(ctx contracts.CallContext, fd int, size int64) error {
	f, err := i.file(ctx, fd)
	if err != nil {
		return err
	}
	if !f.write {
		return syscall.EBADF
	}
	if size < 0 {
		return syscall.EINVAL
	}
	stat, err := i.model.Statfd(f.inode)
	if err != nil {
		return err
	}
	if stat.IsDir {
		return syscall.EISDIR
	}
	if i.machine.readOnly {
		return syscall.EROFS
	}
	return i.model.Truncate(f.inode, size, i.machine.config.CapacityBytes)
}

func (i *Instance) Chdir(ctx contracts.CallContext, path string) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	inode, err := i.model.Getdirinode(i.workingDir, path)
	if err != nil {
		return err
	}
	i.workingDir = inode
	return nil
}

func (i *Instance) Getcwd(ctx contracts.CallContext, buffer abi.ByteView) (int, error) {
	if err := i.validateCall(ctx); err != nil {
		return 0, err
	}
	path, err := i.model.Getpath(i.workingDir)
	if err != nil {
		return 0, err
	}
	if len(path)+1 > buffer.Len() {
		return 0, syscall.EINVAL
	}
	n := buffer.Write([]byte(path))
	buffer.Slice(n, n+1).Write([]byte{0})
	return n + 1, nil
}

func (i *Instance) Mkdirat(ctx contracts.CallContext, dirfd int, path string, mode uint32) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	dir, err := i.dirInode(dirfd)
	if err != nil {
		return err
	}
	return i.model.Mkdir(dir, path)
}

func (i *Instance) Flock(ctx contracts.CallContext, fd, how int) error {
	if _, err := i.file(ctx, fd); err != nil {
		return err
	}
	// TODO(not planned): locks/ownership/contention. This inherited stub takes no lock.
	return nil
}

func (i *Instance) FcntlInt(ctx contracts.CallContext, fd, command, arg int) (int, error) {
	if _, err := i.file(ctx, fd); err != nil {
		return 0, err
	}
	return 0, syscall.ENOSYS
}

func (i *Instance) FcntlLock(ctx contracts.CallContext, fd, command int, lock *contracts.Lock) error {
	if _, err := i.file(ctx, fd); err != nil {
		return err
	}
	// TODO(not planned): POSIX/OFD locks. This inherited stub takes no lock.
	return nil
}

func (i *Instance) Map(ctx contracts.CallContext, addr, length uintptr, prot, flags, fd int, offset int64) (uintptr, error) {
	f, err := i.file(ctx, fd)
	if err != nil {
		return 0, err
	}
	// TODO(not planned): mappings above 16 MiB, writable/private/anonymous mappings.
	if addr != 0 || length == 0 || length > maxMapping || prot != protRead || flags&^mapPopulate != mapShared || offset != 0 {
		return 0, syscall.EINVAL
	}
	stat, err := i.model.Statfd(f.inode)
	if err != nil {
		return 0, err
	}
	if stat.IsDir {
		return 0, syscall.EISDIR
	}
	mapping := i.model.Mmap(f.inode, int(length))
	address := uintptr(unsafe.Pointer(unsafe.SliceData(mapping.Data.Ptr)))
	if i.mappings == nil {
		i.mappings = make(map[uintptr]*model.Mmap)
	}
	i.mappings[address] = mapping
	i.mappingOrder = append(i.mappingOrder, address)
	return address, nil
}

func (i *Instance) Unmap(ctx contracts.CallContext, addr, length uintptr) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	mapping := i.mappings[addr]
	if mapping == nil {
		return syscall.EINVAL
	}
	// Existing API releases the whole mapping by start address, ignoring length.
	i.model.Munmap(mapping)
	delete(i.mappings, addr)
	i.mappingOrder = slices.DeleteFunc(i.mappingOrder, func(address uintptr) bool { return address == addr })
	return nil
}

func (i *Instance) Advise(ctx contracts.CallContext, data abi.ByteView, advice int) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	// TODO(not planned): memory advice semantics/performance. Inherited no-op.
	return nil
}
