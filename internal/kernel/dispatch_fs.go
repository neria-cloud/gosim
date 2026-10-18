package kernel

import (
	"fmt"
	"math"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func (k *Kernel) Filesystem(ctx contracts.CallContext) (contracts.FileOps, error) {
	k.Require(ctx, contracts.FS, "filesystem syscall")
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return nil, err
	}
	if run.bindings.Files == nil {
		return nil, fmt.Errorf("filesystem service missing")
	}
	return run.bindings.Files, nil
}

// TODO(migration): Task 14 generates these typed route decoders and manifest.
// The selected Linux adapters will marshal Stat/Lock and validate the common
// dirent64 wire layout; providers never retain architecture-specific structs.
//
//go:norace
func (d *Dispatcher) decodeFiles(inv *Invocation) (Result, bool, error) {
	ctx, a := inv.Context, inv.Scalars
	var text1, text2 string
	var view abi.ByteView
	var lock abi.ValueView[contracts.Lock]
	used := 0
	valid := true
	switch inv.Route {
	case RouteOpenat, RouteStatat, RouteUnlinkat, RouteChdir, RouteMkdirat:
		used = 1
		text1, valid = inv.Args[0].(string)
	case RouteRenameat:
		used = 2
		var ok bool
		text1, valid = inv.Args[0].(string)
		text2, ok = inv.Args[1].(string)
		valid = valid && ok
	case RouteGetdents64, RoutePread, RoutePwrite, RouteGetcwd, RouteAdvise:
		used = 1
		view, valid = inv.Args[0].(abi.ByteView)
	case RouteFcntlLock:
		used = 1
		lock, valid = inv.Args[0].(abi.ValueView[contracts.Lock])
		valid = valid && lock.UnsafePointer() != nil
	}
	for _, arg := range inv.Args[used:] {
		if arg != nil {
			valid = false
		}
	}
	if !valid {
		return Result{}, false, &contracts.Failure{Code: contracts.InvalidABI, Diagnostic: contracts.Diagnostic{Feature: "fs", Operation: "decode", Epoch: ctx.Epoch, Message: "invalid typed filesystem payload"}}
	}
	files, err := d.kernel.Filesystem(ctx)
	if err != nil {
		return Result{}, false, err
	}
	var result Result
	var n int
	switch inv.Route {
	case RouteOpenat:
		if a[2] > math.MaxUint32 {
			return Result{}, false, fmt.Errorf("open mode exceeds uint32")
		}
		n, err = files.Openat(ctx, int(a[0]), text1, int(a[1]), uint32(a[2]))
		result.R0 = uintptr(n)
	case RouteRenameat:
		err = files.Renameat(ctx, int(a[0]), text1, int(a[1]), text2)
	case RouteGetdents64:
		n, err = files.Getdents64(ctx, int(a[0]), view)
		result.R0 = uintptr(n)
	case RouteFallocate:
		if a[1] > math.MaxUint32 {
			return Result{}, false, fmt.Errorf("fallocate mode exceeds uint32")
		}
		err = files.Fallocate(ctx, int(a[0]), uint32(a[1]), int64(a[2]), int64(a[3]))
	case RouteSeek:
		var pos int64
		pos, err = files.Seek(ctx, int(a[0]), int64(a[1]), int(a[2]))
		result.R0 = uintptr(pos)
	case RoutePwrite:
		n, err = files.Pwrite(ctx, int(a[0]), view, int64(a[1]))
		result.R0 = uintptr(n)
	case RoutePread:
		n, err = files.Pread(ctx, int(a[0]), view, int64(a[1]))
		result.R0 = uintptr(n)
	case RouteSync:
		if a[1] > 1 {
			return Result{}, false, fmt.Errorf("sync data-only flag is not boolean")
		}
		err = files.Sync(ctx, int(a[0]), a[1] != 0)
		if request, ok := err.(*contracts.CrashRequest); ok {
			return Result{}, true, request
		}
	case RouteFstat:
		result.Value, err = files.Fstat(ctx, int(a[0]))
	case RouteStatat:
		result.Value, err = files.Statat(ctx, int(a[0]), text1, int(a[1]))
	case RouteUnlinkat:
		err = files.Unlinkat(ctx, int(a[0]), text1, int(a[1]))
	case RouteTruncate:
		err = files.Truncate(ctx, int(a[0]), int64(a[1]))
	case RouteChdir:
		err = files.Chdir(ctx, text1)
	case RouteGetcwd:
		n, err = files.Getcwd(ctx, view)
		result.R0 = uintptr(n)
	case RouteMkdirat:
		if a[1] > math.MaxUint32 {
			return Result{}, false, fmt.Errorf("mkdir mode exceeds uint32")
		}
		err = files.Mkdirat(ctx, int(a[0]), text1, uint32(a[1]))
	case RouteFlock:
		err = files.Flock(ctx, int(a[0]), int(a[1]))
	case RouteFcntlLock:
		value := lock.Get()
		err = files.FcntlLock(ctx, int(a[0]), int(a[1]), &value)
		if err == nil {
			lock.Set(value)
		}
	case RouteMap:
		result.R0, err = files.Map(ctx, a[0], a[1], int(a[2]), int(a[3]), int(a[4]), int64(a[5]))
	case RouteUnmap:
		err = files.Unmap(ctx, a[0], a[1])
	case RouteAdvise:
		err = files.Advise(ctx, view, int(a[0]))
	}
	if err != nil {
		errno, ok := err.(syscall.Errno)
		if !ok {
			return Result{}, false, fmt.Errorf("unmodeled filesystem syscall error: %w", err)
		}
		result.Errno = uintptr(errno)
	}
	return result, false, nil
}
