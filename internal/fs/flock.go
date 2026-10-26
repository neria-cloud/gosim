package fs

import (
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
)

// These flock constants are shared by the Linux source ABI and Darwin.
const (
	lockShared    = 1
	lockExclusive = 2
	lockNonblock  = 4
	lockUnlock    = 8
)

func (i *Instance) Flock(ctx contracts.CallContext, fd, how int) error {
	f, err := i.file(ctx, fd)
	if err != nil {
		return err
	}
	mode := how &^ lockNonblock
	if how&^(lockShared|lockExclusive|lockNonblock|lockUnlock) != 0 || (mode != lockShared && mode != lockExclusive && mode != lockUnlock) {
		return syscall.EINVAL
	}
	if mode == lockUnlock {
		i.releaseFlock(f)
		return nil
	}
	if mode == lockShared {
		return unsupportedLock(ctx, "flock", "shared flock is not modeled")
	}
	owner := i.flocks[f.inode]
	if owner != nil && owner != f {
		if how&lockNonblock == 0 {
			return unsupportedLock(ctx, "flock", "blocking flock wait is not modeled")
		}
		return syscall.EWOULDBLOCK
	}
	if owner == f {
		return nil
	}
	if i.flocks == nil {
		i.flocks = make(map[int]*File)
	}
	i.flocks[f.inode] = f
	return nil
}

func (i *Instance) releaseFlock(owner *File) {
	if owner.instance == i && i.flocks[owner.inode] == owner {
		delete(i.flocks, owner.inode)
	}
}

func unsupportedLock(ctx contracts.CallContext, operation, detail string) error {
	return &contracts.Failure{Code: contracts.UnsupportedOperation, Diagnostic: contracts.Diagnostic{
		Feature: "fs", Operation: operation, Epoch: ctx.Epoch, PC: ctx.PC, Message: detail,
	}}
}
