//go:build go1.25 && !go1.26

package kernel

import (
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	kernel "github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

const (
	pollNotPollable   = 3
	pollReadMode      = 114
	simulatedPageSize = 4096
	maxWritevVectors  = 1024
)

func InternalPoll_runtimeNano() int64              { return simrt.Nanotime() }
func InternalPoll_runtime_Semacquire(addr *uint32) { simrt.Semacquire(addr, false) }
func InternalPoll_runtime_Semrelease(addr *uint32) { simrt.Semrelease(addr) }
func InternalPoll_runtime_pollServerInit() {
	// Registry is already owned by the bound kernel; no native epoll/kqueue fd.
	Require(contracts.RT, "poll-server-init")
}
func InternalPoll_runtime_isPollServerDescriptor(fd uintptr) bool { return false }
func InternalPoll_runtime_pollOpen(fd uintptr) (uintptr, int) {
	result, err := Call(kernel.PollOpenRequest{FD: int(fd)}, kernel.DecodePollOpenResult)
	if errno, ok := err.(syscall.Errno); ok {
		return 0, int(errno)
	}
	if err != nil {
		Reject("poll-open")
	}
	if result.Code == pollNotPollable {
		return 0, int(syscall.EPERM)
	}
	return uintptr(result.Handle), result.Code
}

func InternalPoll_runtime_pollClose(ctx uintptr) {
	checked(Call(kernel.PollCloseRequest{Handle: uint64(ctx)}, kernel.DecodePollCloseResult))
}

func InternalPoll_runtime_pollWait(ctx uintptr, mode int) int {
	return checked(Call(kernel.PollWaitRequest{Handle: uint64(ctx), Mode: pollMode(mode)}, kernel.DecodePollWaitResult)).Code
}

func InternalPoll_runtime_pollReset(ctx uintptr, mode int) int {
	return checked(Call(kernel.PollResetRequest{Handle: uint64(ctx), Mode: pollMode(mode)}, kernel.DecodePollResetResult)).Code
}

func InternalPoll_runtime_pollSetDeadline(ctx uintptr, d int64, mode int) {
	checked(Call(kernel.PollDeadlineRequest{Handle: uint64(ctx), DeadlineNS: d, Mode: pollMode(mode)}, kernel.DecodePollDeadlineResult))
}

func InternalPoll_runtime_pollUnblock(ctx uintptr) {
	checked(Call(kernel.PollUnblockRequest{Handle: uint64(ctx)}, kernel.DecodePollUnblockResult))
}

func InternalPoll_runtime_pollWaitCanceled(ctx uintptr, mode int) {
	Reject("internal/poll.runtime_pollWaitCanceled")
}

func pollMode(mode int) uint8 {
	if mode != pollReadMode && mode != 119 && mode != 233 {
		Reject("invalid poll mode")
	}
	return uint8(mode)
}
