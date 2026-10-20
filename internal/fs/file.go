package fs

import (
	"math"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// File owns one model inode reference after a successful descriptor transfer.
type File struct {
	instance    *Instance
	inode       int
	pos         int64
	read, write bool
	didReaddir  bool
	released    bool
	closeReason contracts.CloseReason
}

func (*File) Owner() contracts.Feature { return contracts.FS }
func (f *File) Release(reason contracts.CloseReason) error {
	if f.released {
		return nil
	}
	if reason > contracts.CloseAbort {
		return contracts.ErrInvalidConfig
	}
	f.released, f.closeReason = true, reason
	return f.instance.model.CloseFile(f.inode)
}

func (f *File) check(ctx contracts.CallContext) error {
	if err := f.instance.validateCall(ctx); err != nil {
		return err
	}
	if f.released {
		return syscall.EBADF
	}
	return nil
}

func (f *File) readAt(ctx contracts.CallContext, buffer abi.ByteView, offset int64) (int, error) {
	if err := f.check(ctx); err != nil {
		return 0, err
	}
	if !f.read {
		return 0, syscall.EBADF
	}
	if offset < 0 || offset > math.MaxInt64-int64(buffer.Len()) {
		return 0, syscall.EINVAL
	}
	stat, err := f.instance.model.Statfd(f.inode)
	if err != nil {
		return 0, err
	}
	if stat.IsDir {
		return 0, syscall.EISDIR
	}
	return f.instance.model.Read(f.inode, offset, buffer), nil
}

func (f *File) writeAt(ctx contracts.CallContext, buffer abi.ByteView, offset int64) (int, error) {
	if err := f.check(ctx); err != nil {
		return 0, err
	}
	if !f.write {
		return 0, syscall.EBADF
	}
	if offset < 0 || offset > math.MaxInt64-int64(buffer.Len()) {
		return 0, syscall.EINVAL
	}
	stat, err := f.instance.model.Statfd(f.inode)
	if err != nil {
		return 0, err
	}
	if stat.IsDir {
		return 0, syscall.EISDIR
	}
	if f.instance.machine.readOnly {
		return 0, syscall.EROFS
	}
	return f.instance.model.Write(f.inode, offset, buffer, f.instance.machine.config.CapacityBytes)
}

func (f *File) Read(ctx contracts.CallContext, buffer abi.ByteView) (int, error) {
	n, err := f.readAt(ctx, buffer, f.pos)
	f.pos += int64(n)
	return n, err
}

func (f *File) Write(ctx contracts.CallContext, buffer abi.ByteView) (int, error) {
	n, err := f.writeAt(ctx, buffer, f.pos)
	f.pos += int64(n)
	return n, err
}

func (i *Instance) file(ctx contracts.CallContext, fd int) (*File, error) {
	if err := i.validateCall(ctx); err != nil {
		return nil, err
	}
	resource, err := i.fds.Lookup(fd)
	if err != nil {
		return nil, err
	}
	file, ok := resource.(*File)
	if !ok || file.instance != i || file.released {
		return nil, syscall.EBADF
	}
	return file, nil
}
