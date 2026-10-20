package contracts

import "github.com/neria-cloud/gosim/rt/runtime/abi"

type Stat struct {
	Inode                     uint64
	Size                      int64
	Mode                      uint32
	UID, GID                  uint32
	Nlink                     uint64
	AtimeNS, MtimeNS, CtimeNS int64
}
type Lock struct {
	Type, Whence int16
	Start, Len   int64
	PID          int32
}
type FileOps interface {
	Openat(CallContext, int, string, int, uint32) (int, error)
	Renameat(CallContext, int, string, int, string) error
	Getdents64(CallContext, int, abi.ByteView) (int, error)
	Fallocate(CallContext, int, uint32, int64, int64) error
	Seek(CallContext, int, int64, int) (int64, error)
	Pwrite(CallContext, int, abi.ByteView, int64) (int, error)
	Pread(CallContext, int, abi.ByteView, int64) (int, error)
	Sync(CallContext, int, bool) error // bool=dataOnly; current semantics may coincide
	Fstat(CallContext, int) (Stat, error)
	Statat(CallContext, int, string, int) (Stat, error)
	StatFS(CallContext, string) (FSStorageStats, error)
	FstatFS(CallContext, int) (FSStorageStats, error)
	Unlinkat(CallContext, int, string, int) error
	Truncate(CallContext, int, int64) error
	Chdir(CallContext, string) error
	Getcwd(CallContext, abi.ByteView) (int, error)
	Mkdirat(CallContext, int, string, uint32) error
	Flock(CallContext, int, int) error
	FcntlInt(CallContext, int, int, int) (int, error)
	FcntlLock(CallContext, int, int, *Lock) error
	Map(CallContext, uintptr, uintptr, int, int, int, int64) (uintptr, error)
	Unmap(CallContext, uintptr, uintptr) error
	Advise(CallContext, abi.ByteView, int) error
}
