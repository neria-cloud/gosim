package fs

import (
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// TODO(migration): Task 9 replaces every unfinished FileOps method with the
// existing syscall semantics. The normal composition is not enabled yet.
func (i *Instance) unfinished(ctx contracts.CallContext, operation string) error {
	if err := i.validateCall(ctx); err != nil {
		return err
	}
	return &contracts.Failure{Code: contracts.InvariantFailure, Diagnostic: contracts.Diagnostic{Feature: "fs", Operation: operation, Epoch: ctx.Epoch, Message: "FileOps implementation pending Task 9"}}
}

func (i *Instance) Openat(ctx contracts.CallContext, _ int, _ string, _ int, _ uint32) (int, error) {
	return 0, i.unfinished(ctx, "Openat")
}

func (i *Instance) Renameat(ctx contracts.CallContext, _ int, _ string, _ int, _ string) error {
	return i.unfinished(ctx, "Renameat")
}

func (i *Instance) Getdents64(ctx contracts.CallContext, _ int, _ abi.ByteView) (int, error) {
	return 0, i.unfinished(ctx, "Getdents64")
}

func (i *Instance) Fallocate(ctx contracts.CallContext, _ int, _ uint32, _ int64, _ int64) error {
	return i.unfinished(ctx, "Fallocate")
}

func (i *Instance) Seek(ctx contracts.CallContext, _ int, _ int64, _ int) (int64, error) {
	return 0, i.unfinished(ctx, "Seek")
}

func (i *Instance) Pwrite(ctx contracts.CallContext, _ int, _ abi.ByteView, _ int64) (int, error) {
	return 0, i.unfinished(ctx, "Pwrite")
}

func (i *Instance) Pread(ctx contracts.CallContext, _ int, _ abi.ByteView, _ int64) (int, error) {
	return 0, i.unfinished(ctx, "Pread")
}

func (i *Instance) Sync(ctx contracts.CallContext, _ int, _ bool) error {
	return i.unfinished(ctx, "Sync")
}

func (i *Instance) Fstat(ctx contracts.CallContext, _ int) (contracts.Stat, error) {
	return contracts.Stat{}, i.unfinished(ctx, "Fstat")
}

func (i *Instance) Statat(ctx contracts.CallContext, _ int, _ string, _ int) (contracts.Stat, error) {
	return contracts.Stat{}, i.unfinished(ctx, "Statat")
}

func (i *Instance) Unlinkat(ctx contracts.CallContext, _ int, _ string, _ int) error {
	return i.unfinished(ctx, "Unlinkat")
}

func (i *Instance) Truncate(ctx contracts.CallContext, _ int, _ int64) error {
	return i.unfinished(ctx, "Truncate")
}

func (i *Instance) Chdir(ctx contracts.CallContext, _ string) error {
	return i.unfinished(ctx, "Chdir")
}

func (i *Instance) Getcwd(ctx contracts.CallContext, _ abi.ByteView) (int, error) {
	return 0, i.unfinished(ctx, "Getcwd")
}

func (i *Instance) Mkdirat(ctx contracts.CallContext, _ int, _ string, _ uint32) error {
	return i.unfinished(ctx, "Mkdirat")
}

func (i *Instance) Flock(ctx contracts.CallContext, _ int, _ int) error {
	return i.unfinished(ctx, "Flock")
}

func (i *Instance) FcntlInt(ctx contracts.CallContext, _ int, _ int, _ int) (int, error) {
	return 0, i.unfinished(ctx, "FcntlInt")
}

func (i *Instance) FcntlLock(ctx contracts.CallContext, _ int, _ int, _ *contracts.Lock) error {
	return i.unfinished(ctx, "FcntlLock")
}

func (i *Instance) Map(ctx contracts.CallContext, _ uintptr, _ uintptr, _ int, _ int, _ int, _ int64) (uintptr, error) {
	return 0, i.unfinished(ctx, "Map")
}

func (i *Instance) Unmap(ctx contracts.CallContext, _ uintptr, _ uintptr) error {
	return i.unfinished(ctx, "Unmap")
}

func (i *Instance) Advise(ctx contracts.CallContext, _ abi.ByteView, _ int) error {
	return i.unfinished(ctx, "Advise")
}
