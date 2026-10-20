package kernel

import (
	"errors"
	"fmt"
	"math"
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

// controlErrorResult contains copied diagnostic values, never a provider-owned
// error object. Completion is deliberately not application synchronization.
type controlErrorResult struct {
	message string
	cause   contracts.Sentinel
}

func (e controlErrorResult) Error() string { return e.message }
func (e controlErrorResult) Unwrap() error {
	if e.cause == "" {
		return nil
	}
	return e.cause
}

func copyControlError(err error) controlErrorResult {
	result := controlErrorResult{message: err.Error()}
	for _, cause := range []error{
		contracts.ErrNotSim, contracts.ErrWrongRun, contracts.ErrMachineNotFound,
		contracts.ErrInvalidState, contracts.ErrNotStarted, contracts.ErrSelfTransition,
		contracts.ErrInvalidConfig, contracts.ErrEnumerationLimit,
	} {
		if errors.Is(err, cause) {
			result.cause = cause.(contracts.Sentinel)
			break
		}
	}
	return result
}

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
	timers    map[uint64]contracts.EventID
}

func NewDispatcher(kernel *Kernel) *Dispatcher {
	return &Dispatcher{
		kernel: kernel, incoming: simrt.NewChan[*Invocation](0), nextID: 1,
		live: make(
			map[uint64]*invocationLease,
		), envelopes: make(map[*Invocation]uint64),
		canceled: make(map[uint64]contracts.EpochRef), timers: make(map[uint64]contracts.EventID),
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
	if err, ok := inv.Result.Value.(controlErrorResult); ok {
		inv.Result.Value = nil
		// Copy and box on the caller before instrumented Error/Unwrap reads.
		return err
	}
	if errno, ok := inv.Result.Value.(syscall.Errno); ok {
		inv.Result.Value = nil
		return errno // modeled numeric errors already have value ownership
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
	if simrt.TraceSyscall.Enabled() {
		k.engine.WriteTrace(
			tokenFor(inv.Context.Epoch),
			inv.Context.GoroutineID,
			inv.Context.PC,
			"call "+inv.Route.String(),
		)
	}
	previous := k.caller
	k.caller = inv.Context
	defer func() { k.caller = previous }()
	var result Result
	pending := false
	err := k.protected(func() error {
		var err error
		openMutation := d.openatMutates(inv)
		result, pending, err = d.decode(inv)
		if err == nil && !pending {
			var delay int64
			delay, err = d.fsDelay(inv, openMutation)
			if err == nil && delay > 0 {
				pending, err = d.scheduleFSCompletion(inv, result, delay)
			}
		}
		return err
	})
	if request, ok := err.(*contracts.CrashRequest); ok {
		if inv.Route != RouteSync || request.Epoch != inv.Context.Epoch || !k.Accepts(request.Epoch) {
			k.invariant("invalid provider crash request")
			return
		}
		var stopErr error
		if request.Epoch.Machine.ID == k.root {
			k.closeKind = contracts.StopCrash
			stopErr = k.Close()
		} else {
			stopErr = k.stop(k.machineByID[request.Epoch.Machine.ID], contracts.StopCrash)
		}
		if stopErr != nil {
			k.cleanupFailure("sync fault cleanup", stopErr)
		}
		return // cancellation removed this invocation; never wake the caller
	}
	if failure, ok := err.(*contracts.Failure); ok {
		diagnostic := failure.Diagnostic
		diagnostic.Kind = string(failure.Code)
		k.AbortRun(diagnostic)
		return
	}
	if err != nil {
		k.cleanupFailure("syscall dispatch", err)
		return
	}
	if !pending {
		_ = d.Complete(inv.Context.InvocationID, result)
	}
}

//go:norace
func clearInvocation(inv *Invocation) {
	inv.Args, inv.Scalars, inv.Result = [invocationArgCount]any{}, [invocationScalarCount]uintptr{}, Result{}
}

const (
	invocationArgCount    = 5
	invocationScalarCount = 6
	linuxOpenTruncate     = 0x200
	linuxOpenCreate       = 0x40
)

// fsDelay classifies the completed operation. Descriptor read/write is delayed
// only when the descriptor belongs to the filesystem provider.
func (d *Dispatcher) openatMutates(inv *Invocation) bool {
	if inv.Route != RouteOpenat {
		return false
	}
	p, ok := inv.Args[0].(OpenatRequest)
	if !ok {
		return false // decode reports malformed payload
	}
	if p.Flags&linuxOpenTruncate != 0 { // Linux O_TRUNC
		return true
	}
	if p.Flags&linuxOpenCreate == 0 { // Linux O_CREAT
		return false
	}
	files, err := d.kernel.Filesystem(inv.Context)
	if err != nil {
		return false // decode handles unavailable services
	}
	_, err = files.Statat(inv.Context, p.DirFD, p.Path, 0)
	return errors.Is(err, syscall.ENOENT)
}

func (d *Dispatcher) fsDelay(inv *Invocation, openMutation bool) (int64, error) {
	switch inv.Route {
	case RouteRead, RouteWrite:
		var fd int
		if inv.Route == RouteRead {
			fd = inv.Args[0].(ReadRequest).FD
		} else {
			fd = inv.Args[0].(WriteRequest).FD
		}
		run, err := d.kernel.activeEpoch(inv.Context.Epoch)
		if err != nil {
			return 0, err
		}
		resource, err := run.descriptors.Lookup(fd)
		if err != nil || resource.Owner() != contracts.FS {
			return 0, nil
		}
	case RouteOpenat:
		if !openMutation {
			return 0, nil
		}
	case RoutePread, RoutePwrite, RouteFallocate, RouteTruncate, RouteSync,
		RouteMkdirat, RouteRenameat, RouteUnlinkat, RouteFchmod, RouteChmodat,
		RouteFchown, RouteChownat:
	default:
		return 0, nil
	}
	files, err := d.kernel.Filesystem(inv.Context)
	if err != nil {
		return 0, err
	}
	return files.StorageDelayNS(inv.Context), nil
}

// scheduleFSCompletion retains only the scalar result and epoch identity.
// Borrowed ByteViews leave the supervisor envelope before any clock event runs.
func (d *Dispatcher) scheduleFSCompletion(inv *Invocation, result Result, delay int64) (bool, error) {
	if delay <= 0 {
		return false, nil
	}
	now := d.kernel.Ports().Clock.NowNanos()
	if now > math.MaxInt64-delay {
		return false, contracts.ErrInvalidConfig
	}
	id, epoch := inv.Context.InvocationID, inv.Context.Epoch
	clearInvocation(inv)
	event, err := d.kernel.Ports().Events.Schedule(epoch, now+delay, func() {
		if !d.ownsInvocation(contracts.CallContext{Epoch: epoch, InvocationID: id}) {
			return
		}
		delete(d.timers, id)
		_ = d.Complete(id, result)
	})
	if err != nil {
		return false, err
	}
	d.timers[id] = event
	return true, nil
}

// Complete drops borrowed arguments and the lease before waking a caller.
// Missing issued IDs mean duplicate completion unless cancellation metadata
// explicitly admits a stale callback; no ID is ever reused.
//
//go:norace
func (d *Dispatcher) Complete(id uint64, result Result) error {
	if event, ok := d.timers[id]; ok {
		d.kernel.Ports().Events.Cancel(event)
		delete(d.timers, id)
	}
	if epoch, ok := d.canceled[id]; ok {
		d.kernel.Ports().Trace.Emit(
			epoch,
			"discard-canceled-completion",
			[]contracts.Field{{Key: "invocation", Value: fmt.Sprint(id)}},
		)
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
	if simrt.TraceSyscall.Enabled() {
		d.kernel.engine.WriteTrace(
			tokenFor(inv.Context.Epoch),
			inv.Context.GoroutineID,
			inv.Context.PC,
			"ret  "+inv.Route.String(),
		)
	}
	clearInvocation(inv)
	if err, ok := result.Value.(error); ok {
		if _, numeric := err.(syscall.Errno); !numeric {
			result.Value = copyControlError(err)
		}
	}
	inv.Result = result
	delete(d.live, id)
	delete(d.envelopes, inv)
	entry.invocation = nil
	d.kernel.engine.WakeSemaphore(abi.NewValueView(&inv.Sema))
	return nil
}

//go:norace
func (d *Dispatcher) cancel(id uint64, entry *invocationLease) {
	if event, ok := d.timers[id]; ok {
		d.kernel.Ports().Events.Cancel(event)
		delete(d.timers, id)
	}
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
	if len(d.live) != 0 || len(d.envelopes) != 0 || len(d.timers) != 0 || len(d.kernel.waits) != 0 {
		return errors.New("dispatcher retained borrowed invocations or observers")
	}
	clear(d.canceled)
	return nil
}
