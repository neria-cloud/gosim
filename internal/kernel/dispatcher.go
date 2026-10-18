package kernel

import (
	"errors"
	"fmt"
	"slices"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type Invocation struct {
	Context contracts.CallContext
	Route   RouteID
	Scalars [6]uintptr
	Args    [5]any
	Result  Result
	Sema    uint32
}
type (
	RouteID uint32
	Result  struct {
		R0, R1 uintptr
		Value  any
		Errno  uintptr
	}
)

// TODO(migration): generate the complete typed route manifest/decoders in Task
// 14. These core routes are explicit; every unlisted route fails the run.
const (
	RouteRead RouteID = iota + 1
	RouteWrite
	RouteClose
	RouteFcntlInt
	RouteGetrandom
	RouteGetpid
	RouteUname
	RoutePollOpen
	RoutePollClose
	RoutePollWait
	RoutePollReset
	RoutePollDeadline
	RoutePollUnblock
	RouteWait
)

type invocationLease struct {
	epoch      contracts.EpochRef
	invocation *Invocation
}
type Dispatcher struct {
	kernel    *Kernel
	incoming  simrt.Chan[*Invocation]
	nextID    uint64
	live      map[uint64]*invocationLease
	envelopes map[*Invocation]uint64
	canceled  map[uint64]contracts.EpochRef
}

func NewDispatcher(kernel *Kernel) *Dispatcher {
	return &Dispatcher{
		kernel: kernel, incoming: simrt.NewChan[*Invocation](0), nextID: 1,
		live: make(map[uint64]*invocationLease), envelopes: make(map[*Invocation]uint64), canceled: make(map[uint64]contracts.EpochRef),
	}
}

// Submit is called by application transport. The unbuffered channel transfers
// ownership to the supervisor. Native channel generics preserve the translated
// envelope's type identity; no original-domain invocation is constructed.
//
//go:norace
func (d *Dispatcher) Submit(inv *Invocation) error {
	if inv == nil {
		return contracts.ErrInvalidConfig
	}
	token := simrt.CurrentMachineToken()
	if token.ID == 0 || inv.Context.Epoch.Machine.RunID != d.kernel.runID || tokenFor(inv.Context.Epoch) != token {
		return contracts.ErrInvalidState
	}
	simrt.SyscallEnter()
	defer simrt.SyscallExit()
	inv.Context.InvocationID, inv.Context.GoroutineID = 0, uint64(simrt.GetGoroutine())
	inv.Sema, inv.Result = 0, Result{}
	d.incoming.Send(inv)
	simrt.Semacquire(&inv.Sema, false)
	if err, ok := inv.Result.Value.(error); ok {
		inv.Result.Value = nil
		return err
	}
	return nil
}

// TakeResult transfers the result and clears the reusable envelope.
//
//go:norace
func (inv *Invocation) TakeResult() Result {
	result := inv.Result
	inv.Result = Result{}
	return result
}

//go:norace
func (d *Dispatcher) admit(inv *Invocation) error {
	if inv == nil {
		return contracts.ErrInvalidConfig
	}
	if !d.kernel.Accepts(inv.Context.Epoch) {
		return contracts.ErrInvalidState
	}
	if d.envelopes[inv] != 0 {
		return errors.New("invocation envelope already leased")
	}
	if d.nextID == 0 {
		return errors.New("invocation ID space exhausted")
	}
	id := d.nextID
	d.nextID++
	inv.Context.InvocationID = id
	d.live[id] = &invocationLease{epoch: inv.Context.Epoch, invocation: inv}
	d.envelopes[inv] = id
	return nil
}

//go:norace
func (d *Dispatcher) ownsInvocation(ctx contracts.CallContext) bool {
	entry := d.live[ctx.InvocationID]
	return entry != nil && entry.epoch == ctx.Epoch
}

// DispatchOne never accepts a caller-supplied trampoline or provider pointer.
//
//go:norace
func (d *Dispatcher) DispatchOne(inv *Invocation) {
	if err := d.admit(inv); err != nil {
		d.kernel.invariant("invocation admission: " + err.Error())
		return
	}
	k := d.kernel
	previous := k.caller
	k.caller = inv.Context
	defer func() { k.caller = previous }()
	var result Result
	pending := false
	err := k.protected(func() error {
		var err error
		result, pending, err = d.decode(inv)
		return err
	})
	if err != nil {
		k.cleanupFailure("syscall dispatch", err)
		return
	}
	if !pending {
		_ = d.Complete(inv.Context.InvocationID, result)
	}
}

//go:norace
func (d *Dispatcher) decode(inv *Invocation) (Result, bool, error) {
	k, ctx, a := d.kernel, inv.Context, inv.Scalars
	usesView := inv.Route == RouteRead || inv.Route == RouteWrite || inv.Route == RouteGetrandom
	var view abi.ByteView
	for i, arg := range inv.Args {
		if i == 0 && usesView {
			var ok bool
			view, ok = arg.(abi.ByteView)
			if !ok {
				k.AbortRun(contracts.Diagnostic{Kind: string(contracts.InvalidABI), Operation: "decode", Epoch: ctx.Epoch, Message: "route requires abi.ByteView"})
				return Result{}, false, nil
			}
		} else if arg != nil {
			k.AbortRun(contracts.Diagnostic{Kind: string(contracts.InvalidABI), Operation: "decode", Epoch: ctx.Epoch, Message: "unexpected invocation payload"})
			return Result{}, false, nil
		}
	}
	var result Result
	var err error
	modeIndex := -1
	if inv.Route == RoutePollWait || inv.Route == RoutePollReset {
		modeIndex = 1
	}
	if inv.Route == RoutePollDeadline {
		modeIndex = 2
	}
	if modeIndex >= 0 && a[modeIndex] > 255 {
		k.AbortRun(contracts.Diagnostic{Kind: string(contracts.InvalidABI), Operation: "decode", Epoch: ctx.Epoch, Message: "poll mode exceeds uint8"})
		return Result{}, false, nil
	}
	switch inv.Route {
	case RouteRead:
		var n int
		n, err = k.Read(ctx, int(a[0]), view)
		result.R0 = uintptr(n)
	case RouteWrite:
		var n int
		n, err = k.Write(ctx, int(a[0]), view)
		result.R0 = uintptr(n)
	case RouteClose:
		err = k.CloseFD(ctx, int(a[0]))
	case RouteFcntlInt:
		var n int
		n, err = k.FcntlInt(ctx, int(a[0]), int(a[1]), int(a[2]))
		result.R0 = uintptr(n)
	case RouteGetrandom:
		var n int
		n, err = k.Getrandom(ctx, view, int(a[0]))
		result.R0 = uintptr(n)
	case RouteGetpid:
		result.R0 = uintptr(k.Getpid(ctx))
	case RouteUname:
		result.Value, err = k.Uname(ctx)
	case RoutePollOpen:
		id, err := k.PollOpen(ctx, int(a[0]))
		result = pollResult(err)
		result.R1 = uintptr(id)
		return result, false, nil
	case RoutePollClose:
		return pollResult(k.PollClose(ctx, uint64(a[0]))), false, nil
	case RoutePollWait:
		err := k.PollWait(ctx, uint64(a[0]), uint8(a[1]))
		if err == errPending {
			return Result{}, true, nil
		}
		return pollResult(err), false, nil
	case RoutePollReset:
		return pollResult(k.PollReset(ctx, uint64(a[0]), uint8(a[1]))), false, nil
	case RoutePollDeadline:
		return pollResult(k.PollDeadline(ctx, uint64(a[0]), int64(a[1]), uint8(a[2]))), false, nil
	case RoutePollUnblock:
		return pollResult(k.PollUnblock(ctx, uint64(a[0]))), false, nil
	case RouteWait:
		id, err := k.Wait(ctx, contracts.MachineRef{RunID: uint64(a[0]), ID: uint64(a[1])})
		if err != nil {
			return Result{Value: err}, false, nil
		}
		wait := k.waits[id]
		if wait.exit.Valid {
			delete(k.waits, id)
			return Result{Value: wait.exit}, false, nil
		}
		return Result{}, true, nil
	default:
		k.AbortRun(contracts.Diagnostic{Kind: string(contracts.UnsupportedOperation), Operation: "dispatch", Epoch: ctx.Epoch, PC: ctx.PC, Message: fmt.Sprintf("unlisted route %d", inv.Route)})
		return Result{}, false, nil
	}
	if err != nil {
		errno, ok := err.(syscall.Errno)
		if !ok {
			return Result{}, false, fmt.Errorf("unmodeled syscall error: %w", err)
		}
		result.Errno = uintptr(errno)
	}
	return result, false, nil
}

//go:norace
func clearInvocation(inv *Invocation) { inv.Args, inv.Result = [5]any{}, Result{} }

// Complete drops borrowed arguments and the lease before waking a caller.
// Missing issued IDs mean duplicate completion unless cancellation metadata
// explicitly admits a stale callback; no ID is ever reused.
//
//go:norace
func (d *Dispatcher) Complete(id uint64, result Result) error {
	if epoch, ok := d.canceled[id]; ok {
		d.kernel.Ports().Trace.Emit(epoch, "discard-canceled-completion", []contracts.Field{{Key: "invocation", Value: fmt.Sprint(id)}})
		return nil
	}
	entry := d.live[id]
	if entry == nil {
		err := fmt.Errorf("unknown or duplicate completion %d", id)
		d.kernel.invariant(err.Error())
		return err
	}
	if !d.kernel.Accepts(entry.epoch) {
		d.cancel(id, entry)
		return nil
	}
	inv := entry.invocation
	clearInvocation(inv)
	inv.Result = result
	delete(d.live, id)
	delete(d.envelopes, inv)
	entry.invocation = nil
	d.kernel.engine.WakeSemaphore(abi.NewValueView(&inv.Sema))
	return nil
}

//go:norace
func (d *Dispatcher) cancel(id uint64, entry *invocationLease) {
	inv := entry.invocation
	clearInvocation(inv)
	d.kernel.engine.RetireSemaphore(abi.NewValueView(&inv.Sema))
	delete(d.live, id)
	delete(d.envelopes, inv)
	d.canceled[id] = entry.epoch
	entry.invocation = nil
}

func (d *Dispatcher) CancelEpoch(epoch contracts.EpochRef) {
	var ids []uint64
	for id, entry := range d.live {
		if entry.epoch == epoch {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		d.cancel(id, d.live[id])
	}
	for id, wait := range d.kernel.waits {
		if wait.caller == epoch {
			delete(d.kernel.waits, id)
		}
	}
}

// releaseEpoch follows provider detach and event cancellation. No valid owner
// callback can then refer to that epoch's canceled leases.
func (d *Dispatcher) releaseEpoch(epoch contracts.EpochRef) {
	for id, owner := range d.canceled {
		if owner == epoch {
			delete(d.canceled, id)
		}
	}
}

func (d *Dispatcher) Drain() error {
	if len(d.live) != 0 || len(d.envelopes) != 0 || len(d.kernel.waits) != 0 {
		return errors.New("dispatcher retained borrowed invocations or observers")
	}
	clear(d.canceled)
	return nil
}
