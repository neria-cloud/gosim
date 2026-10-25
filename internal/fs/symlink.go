package fs

import (
	"strings"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func (i *Instance) Symlinkat(ctx contracts.CallContext, target string, dirfd int, path string) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	if target == "" {
		return syscall.ENOENT
	}
	if strings.IndexByte(target, 0) >= 0 {
		return syscall.EINVAL
	}
	if len(target) >= 4096 {
		return syscall.ENAMETOOLONG
	}
	dir, err := i.dirInode(dirfd, path)
	if err != nil {
		return err
	}
	if err := i.checkMutation(); err != nil {
		return err
	}
	return i.model.Symlink(dir, path, target, i.identity())
}

func (i *Instance) Readlinkat(ctx contracts.CallContext, dirfd int, path string, into abi.ByteView) (int, error) {
	if err := i.validateCall(ctx); err != nil {
		return 0, err
	}
	if into.Len() == 0 {
		return 0, syscall.EINVAL
	}
	dir, err := i.dirInode(dirfd, path)
	if err != nil {
		return 0, err
	}
	target, err := i.model.Readlink(dir, path, i.identity())
	if err != nil {
		return 0, err
	}
	return into.Write([]byte(target)), nil
}
