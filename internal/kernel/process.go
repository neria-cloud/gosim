package kernel

import (
	"encoding/binary"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type (
	ProcessInfo       struct{ Sysname, Nodename, Release, Version, Machine string }
	ProcessCapability struct{ Operation, Status, Detail string }
)

func ProcessCapabilities() []ProcessCapability {
	// TODO(not planned): full Linux process, signal and procfs semantics.
	return []ProcessCapability{
		{"getrandom", "partial", "deterministic application stream; flags=0 only"},
		{"getpid", "stub", "fixed historical PID 42; no Linux process model"},
		{"uname.nodename", "supported", "machine label truncated to 64 bytes"},
		{"uname.other-fields", "missing", "sysname/release/version/machine remain empty"},
		{"signals/procfs", "unsupported", "no host fallback or generic success response"},
	}
}

func tokenFor(epoch contracts.EpochRef) simrt.MachineToken {
	return simrt.MachineToken{ID: epoch.Machine.ID, Epoch: epoch.Epoch}
}

func (k *Kernel) Getrandom(ctx contracts.CallContext, into abi.ByteView, flags int) (int, error) {
	if _, err := k.activeEpoch(ctx.Epoch); err != nil {
		return 0, err
	}
	if flags != 0 {
		return 0, syscall.EINVAL
	}
	random := k.engine.RandomFor("application", ctx.Epoch.Machine.ID)
	n := into.Len()
	for into.Len() != 0 {
		var block [8]byte
		binary.LittleEndian.PutUint64(block[:], random.Uint64())
		written := into.Write(block[:])
		into = into.SliceFrom(written)
	}
	return n, nil
}

func (k *Kernel) Getpid(ctx contracts.CallContext) int {
	if _, err := k.activeEpoch(ctx.Epoch); err != nil {
		k.invariant("getpid on inactive epoch")
		return 0
	}
	return 42
}

func (k *Kernel) Uname(ctx contracts.CallContext) (ProcessInfo, error) {
	if _, err := k.activeEpoch(ctx.Epoch); err != nil {
		return ProcessInfo{}, err
	}
	label := k.machineByID[ctx.Epoch.Machine.ID].label
	return ProcessInfo{Nodename: label[:min(len(label), 64)]}, nil
}
