package kernel

import (
	"errors"
	"math"
	"slices"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// FDTable owns every resource transferred to it, regardless of provider.
type FDTable struct {
	epoch   contracts.EpochRef
	entries map[int]contracts.Resource
	next    int
	closed  bool
}

func NewFDTable(epoch contracts.EpochRef) *FDTable {
	return &FDTable{epoch: epoch, entries: make(map[int]contracts.Resource), next: 3}
}

func (t *FDTable) Allocate(resource contracts.Resource) (int, error) {
	if t.closed || resource == nil {
		return 0, contracts.ErrInvalidState
	}
	if owner := resource.Owner(); owner != contracts.RT && owner != contracts.FS && owner != contracts.Net {
		return 0, contracts.ErrInvalidConfig
	}
	if t.next > math.MaxInt32 {
		return 0, syscall.EMFILE
	}
	fd := t.next
	t.next++
	t.entries[fd] = resource
	return fd, nil
}

func (t *FDTable) Lookup(fd int) (contracts.Resource, error) {
	resource := t.entries[fd]
	if resource == nil {
		return nil, syscall.EBADF
	}
	return resource, nil
}

func (t *FDTable) Remove(fd int) (contracts.Resource, error) {
	resource, err := t.Lookup(fd)
	if err != nil {
		return nil, err
	}
	delete(t.entries, fd)
	return resource, nil
}

func (t *FDTable) Finalize(reason contracts.CloseReason) error {
	if t.closed {
		return nil
	}
	t.closed = true
	var fds []int
	for fd := range t.entries {
		fds = append(fds, fd)
	}
	slices.Sort(fds)
	var failures []error
	for _, fd := range fds {
		resource := t.entries[fd]
		delete(t.entries, fd)
		failures = append(failures, protected(func() error { return resource.Release(reason) }))
	}
	return errors.Join(failures...)
}

type diagnosticResource struct {
	closed bool
	kernel *Kernel
	epoch  contracts.EpochRef
	fd     int
}

func (*diagnosticResource) Owner() contracts.Feature              { return contracts.RT }
func (r *diagnosticResource) Release(contracts.CloseReason) error { r.closed = true; return nil }

func (r *diagnosticResource) Read(_ contracts.CallContext, _ abi.ByteView) (int, error) {
	if r.closed || r.fd != 0 {
		return 0, syscall.EBADF
	}
	return 0, nil // deterministic EOF; never reads host stdin
}

func (r *diagnosticResource) Write(ctx contracts.CallContext, data abi.ByteView) (int, error) {
	if r.closed || r.fd == 0 {
		return 0, syscall.EBADF
	}
	owned := make([]byte, data.Len())
	data.Read(owned)
	event := "stdout"
	if r.fd == 2 {
		event = "stderr"
	}
	r.kernel.Ports().Trace.Emit(r.epoch, event, []contracts.Field{{Key: "message", Value: string(owned)}})
	r.kernel.engine.WriteDiagnostic(tokenFor(r.epoch), ctx.GoroutineID, r.fd, abi.ByteView{Ptr: owned})
	return len(owned), nil
}

func (t *FDTable) installDiagnostics(kernel *Kernel) error {
	if t.closed || len(t.entries) != 0 {
		return contracts.ErrInvalidState
	}
	for fd := 0; fd < 3; fd++ {
		t.entries[fd] = &diagnosticResource{kernel: kernel, epoch: t.epoch, fd: fd}
	}
	return nil
}

func (k *Kernel) activeEpoch(epoch contracts.EpochRef) (*Run, error) {
	if !k.Accepts(epoch) {
		return nil, contracts.ErrInvalidState
	}
	return k.machineByID[epoch.Machine.ID].current, nil
}

func (k *Kernel) Read(ctx contracts.CallContext, fd int, into abi.ByteView) (int, error) {
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return 0, err
	}
	resource, err := run.descriptors.Lookup(fd)
	if err != nil {
		return 0, err
	}
	k.Require(ctx, resource.Owner(), "read")
	readable, ok := resource.(contracts.Readable)
	if !ok {
		return 0, syscall.EBADF
	}
	return readable.Read(ctx, into)
}

func (k *Kernel) Write(ctx contracts.CallContext, fd int, from abi.ByteView) (int, error) {
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return 0, err
	}
	resource, err := run.descriptors.Lookup(fd)
	if err != nil {
		return 0, err
	}
	k.Require(ctx, resource.Owner(), "write")
	writable, ok := resource.(contracts.Writable)
	if !ok {
		return 0, syscall.EBADF
	}
	return writable.Write(ctx, from)
}

func (k *Kernel) CloseFD(ctx contracts.CallContext, fd int) error {
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return err
	}
	resource, err := run.descriptors.Lookup(fd)
	if err != nil {
		return err
	}
	k.Require(ctx, resource.Owner(), "close")
	pollErr := k.closePolls(ctx.Epoch, fd)
	if _, err := run.descriptors.Remove(fd); err != nil {
		return err
	}
	releaseErr := resource.Release(contracts.CloseExplicit)
	if pollErr != nil {
		k.cleanupFailure("descriptor poll teardown", errors.Join(pollErr, releaseErr))
		return errors.Join(pollErr, releaseErr)
	}
	return releaseErr
}

func (k *Kernel) FcntlInt(ctx contracts.CallContext, fd, command, argument int) (int, error) {
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return 0, err
	}
	resource, err := run.descriptors.Lookup(fd)
	if err != nil {
		return 0, err
	}
	k.Require(ctx, resource.Owner(), "fcntl")
	switch resource.Owner() {
	case contracts.RT:
		// Linux F_GETFL during os standard-file initialization. No native poller.
		if fd >= 0 && fd <= 2 && command == 3 && argument == 0 {
			if fd == 0 {
				return 0, nil
			}
			return 1, nil
		}
		k.AbortRun(contracts.Diagnostic{Kind: string(contracts.UnsupportedOperation), Feature: "rt", Operation: "fcntl", Epoch: ctx.Epoch, PC: ctx.PC, Message: "unsupported diagnostic descriptor command"})
		return 0, contracts.ErrInvalidState
	case contracts.FS:
		if run.bindings.Files == nil {
			k.invariant("filesystem descriptor has no service")
			return 0, contracts.ErrInvalidState
		}
		return run.bindings.Files.FcntlInt(ctx, fd, command, argument)
	case contracts.Net:
		if run.bindings.Sockets == nil {
			k.invariant("network descriptor has no service")
			return 0, contracts.ErrInvalidState
		}
		return run.bindings.Sockets.FcntlInt(ctx, fd, command, argument)
	default:
		k.AbortRun(contracts.Diagnostic{Kind: string(contracts.UnsupportedOperation), Feature: "rt", Operation: "fcntl", Epoch: ctx.Epoch, PC: ctx.PC, Message: "diagnostic descriptors do not model fcntl commands"})
		return 0, contracts.ErrInvalidState
	}
}
