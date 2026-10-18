//go:build linux

package go123

import (
	"github.com/neria-cloud/gosim/gosimruntime"
	"github.com/neria-cloud/gosim/internal/simulation"
	"github.com/neria-cloud/gosim/internal/simulation/syscallabi"
)

func InternalPoll_runtimeNano() int64 {
	panic("gosim not implemented")
}

func InternalPoll_runtime_Semacquire(addr *uint32) {
	gosimruntime.Semacquire(addr, false)
}

func InternalPoll_runtime_Semrelease(addr *uint32) {
	gosimruntime.Semrelease(addr)
}

func InternalPoll_runtime_pollServerInit() {
	// nop
}

func InternalPoll_runtime_isPollServerDescriptor(fd uintptr) bool {
	panic("gosim not implemented")
}

// FIXME: returning a pointer here is very suspect, these are uintptrs on the
// other side and GC is ruthless

func InternalPoll_runtime_pollOpen(fd uintptr) (*syscallabi.PollDesc, int) {
	desc := syscallabi.AllocPollDesc(int(fd))
	code := simulation.SyscallPollOpen(desc.FD(), desc)
	return desc, code
}

func InternalPoll_runtime_pollClose(ctx *syscallabi.PollDesc) {
	simulation.SyscallPollClose(ctx.FD(), ctx)
	ctx.Close()
}

func InternalPoll_runtime_pollWaitCanceled(ctx *syscallabi.PollDesc, mode int) {
	panic("not implemented") // only used on windows
}

func InternalPoll_runtime_pollReset(ctx *syscallabi.PollDesc, mode int) int {
	return ctx.Reset(mode)
}

func InternalPoll_runtime_pollWait(ctx *syscallabi.PollDesc, mode int) int {
	return ctx.Wait(mode)
}

func InternalPoll_runtime_pollSetDeadline(ctx *syscallabi.PollDesc, d int64, mode int) {
	ctx.SetDeadline(d, mode)
}

func InternalPoll_runtime_pollUnblock(ctx *syscallabi.PollDesc) {
	ctx.Unblock()
}
