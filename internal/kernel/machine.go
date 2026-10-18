package kernel

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

type State uint8

// StateNew is qualified because the package also declares constructor New.
const (
	StateNew State = iota
	StateStarting
	StateRunning
	StateStopping
	StateStopped
	StateCrashed
	StateDestroyed
)

type Machine struct {
	ref       contracts.MachineRef
	label     string
	main      func()
	state     State
	nextEpoch uint64
	current   *Run
	lastExit  ExitRecord
}

type Run struct {
	ref         contracts.EpochRef
	accepting   bool
	runtimeID   uint64
	descriptors *FDTable
	bindings    contracts.Bindings
	instances   []contracts.MachineInstance
	unpublished []contracts.PreparedMachine // retained after a publication invariant failure
}

type ExitRecord struct {
	Epoch uint64
	Kind  contracts.StopKind
	Valid bool
}

type waitRecord struct {
	caller contracts.EpochRef
	target contracts.EpochRef
	exit   ExitRecord
}

func (k *Kernel) Create(label string, main func()) (contracts.MachineRef, error) {
	if k.closed {
		return contracts.MachineRef{}, contracts.ErrInvalidState
	}
	if main == nil {
		return contracts.MachineRef{}, fmt.Errorf("%w: nil machine main", contracts.ErrInvalidConfig)
	}
	if k.nextMachineID == 0 {
		k.invariant("machine ID space exhausted")
		return contracts.MachineRef{}, contracts.ErrInvalidState
	}
	ref := contracts.MachineRef{RunID: k.runID, ID: k.nextMachineID}
	k.nextMachineID++
	k.machineByID[ref.ID] = &Machine{ref: ref, label: label, main: main, nextEpoch: 1}
	return ref, nil
}

func (k *Kernel) Lookup(ref contracts.MachineRef) (*Machine, error) {
	if ref.RunID != k.runID {
		return nil, contracts.ErrWrongRun
	}
	m := k.machineByID[ref.ID]
	if m == nil || m.state == StateDestroyed {
		return nil, contracts.ErrMachineNotFound
	}
	return m, nil
}

func (k *Kernel) Accepts(ref contracts.EpochRef) bool {
	m, err := k.Lookup(ref.Machine)
	return err == nil && !k.closed && m.state == StateRunning && m.current != nil && m.current.accepting && m.current.ref == ref
}

func (k *Kernel) callerRef() contracts.MachineRef {
	if k.caller.Epoch.Machine.ID != 0 {
		return k.caller.Epoch.Machine
	}
	token := simrt.CurrentMachineToken()
	if token.ID != 0 {
		return contracts.MachineRef{RunID: k.runID, ID: token.ID}
	}
	return contracts.MachineRef{}
}

func (k *Kernel) returningTransition(m *Machine) error {
	if k.callerRef() == m.ref {
		return contracts.ErrSelfTransition
	}
	if m.ref.ID == k.root {
		return fmt.Errorf("%w: protected controller root", contracts.ErrInvalidState)
	}
	return nil
}

func (k *Kernel) Start(ref contracts.MachineRef) error {
	tx, err := k.BeginStart(ref, false)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (k *Kernel) Restart(ref contracts.MachineRef) error {
	tx, err := k.BeginStart(ref, true)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (k *Kernel) Stop(ref contracts.MachineRef, kind contracts.StopKind) error {
	m, err := k.Lookup(ref)
	if err != nil {
		return err
	}
	if err := k.returningTransition(m); err != nil {
		return err
	}
	if kind != contracts.StopGraceful && kind != contracts.StopCrash {
		return contracts.ErrInvalidConfig
	}
	switch m.state {
	case StateStopped, StateCrashed:
		return nil
	case StateNew:
		return contracts.ErrNotStarted
	case StateRunning:
		err := k.stop(m, kind)
		if err != nil {
			k.cleanupFailure("machine stop", err)
		}
		return err
	default:
		return contracts.ErrInvalidState
	}
}

func token(run *Run) simrt.MachineToken {
	return simrt.MachineToken{ID: run.runtimeID, Epoch: run.ref.Epoch}
}

func closeReason(kind contracts.StopKind) contracts.CloseReason {
	if kind == contracts.StopGraceful {
		return contracts.CloseGraceful
	}
	return contracts.CloseCrash
}

// stop is the trusted non-returning-self/teardown route. Its caller is always
// the surviving supervisor, never the application being canceled.
func (k *Kernel) stop(m *Machine, kind contracts.StopKind) error {
	run := m.current
	m.state, run.accepting = StateStopping, false
	k.engine.CancelMachine(token(run))
	k.dispatcher.CancelEpoch(run.ref)
	k.cancelEvents(run.ref)
	var failures []error
	for i := len(run.instances) - 1; i >= 0; i-- {
		if err := k.protected(func() error { run.instances[i].Quiesce(); return nil }); err != nil {
			failures = append(failures, err)
			kind = contracts.StopCrash
		}
	}
	for _, instance := range run.instances {
		if err := k.protected(func() error { return instance.Settle(kind) }); err != nil {
			failures = append(failures, err)
			kind = contracts.StopCrash
		}
	}
	if err := k.protected(func() error { return run.descriptors.Finalize(closeReason(kind)) }); err != nil {
		failures = append(failures, err)
		kind = contracts.StopCrash
	}
	for i := len(run.unpublished) - 1; i >= 0; i-- {
		if err := k.protected(func() error { run.unpublished[i].Abort(); return nil }); err != nil {
			failures = append(failures, err)
			kind = contracts.StopCrash
		}
	}
	run.unpublished = nil
	for i := len(run.instances) - 1; i >= 0; i-- {
		if err := k.protected(func() error { run.instances[i].Detach(); return nil }); err != nil {
			failures = append(failures, err)
			kind = contracts.StopCrash
		}
	}
	k.engine.DisposeMachine(token(run))
	m.current = nil
	m.state = StateStopped
	if kind == contracts.StopCrash {
		m.state = StateCrashed
	}
	m.lastExit = ExitRecord{Epoch: run.ref.Epoch, Kind: kind, Valid: true}
	k.finishWaits(run.ref, m.lastExit)
	run.instances, run.bindings = nil, contracts.Bindings{}
	return errors.Join(failures...)
}

// requestSelfExit is harness-only. It sends primitive machine/epoch/mode
// values to the surviving supervisor and never resumes application execution.
func requestSelfExit(exits simrt.Chan[[3]uint64], token simrt.MachineToken, kind contracts.StopKind) {
	exits.Send([3]uint64{token.ID, token.Epoch, uint64(kind)})
	simrt.NewChan[uint64](0).Recv()
}

func (k *Kernel) Destroy(ref contracts.MachineRef) error {
	// Repeated destruction is idempotent, while lookup/use of its handle fails.
	if ref.RunID == k.runID {
		if m := k.machineByID[ref.ID]; m != nil && m.state == StateDestroyed {
			return nil
		}
	}
	m, err := k.Lookup(ref)
	if err != nil {
		return err
	}
	if err := k.returningTransition(m); err != nil {
		return err
	}
	if m.state != StateNew && m.state != StateStopped && m.state != StateCrashed {
		return contracts.ErrInvalidState
	}
	var failures []error
	for i := len(k.providers) - 1; i >= 0; i-- {
		failures = append(failures, k.protected(func() error { k.providers[i].Destroy(ref); return nil }))
	}
	m.state, m.main = StateDestroyed, nil
	if err := errors.Join(failures...); err != nil {
		k.cleanupFailure("machine destroy", err)
		return err
	}
	return nil
}

func (k *Kernel) SetMain(ref contracts.MachineRef, main func()) error {
	m, err := k.Lookup(ref)
	if err != nil {
		return err
	}
	if err := k.returningTransition(m); err != nil {
		return err
	}
	if m.state != StateNew && m.state != StateStopped && m.state != StateCrashed {
		return contracts.ErrInvalidState
	}
	if main == nil {
		return contracts.ErrInvalidConfig
	}
	m.main = main
	return nil
}

func (k *Kernel) Wait(ctx contracts.CallContext, ref contracts.MachineRef) (contracts.WaitID, error) {
	if !k.Accepts(ctx.Epoch) {
		return 0, contracts.ErrInvalidState
	}
	m, err := k.Lookup(ref)
	if err != nil {
		return 0, err
	}
	if ref == ctx.Epoch.Machine {
		return 0, contracts.ErrSelfTransition
	}
	wait := &waitRecord{caller: ctx.Epoch}
	switch m.state {
	case StateRunning:
		wait.target = m.current.ref
	case StateStopped, StateCrashed:
		wait.target = contracts.EpochRef{Machine: ref, Epoch: m.lastExit.Epoch}
		wait.exit = m.lastExit
	case StateNew:
		return 0, contracts.ErrNotStarted
	default:
		return 0, contracts.ErrInvalidState
	}
	if k.nextWaitID == 0 {
		k.invariant("wait ID space exhausted")
		return 0, contracts.ErrInvalidState
	}
	id := k.nextWaitID
	k.nextWaitID++
	k.waits[id] = wait
	return id, nil
}

func (k *Kernel) finishWaits(epoch contracts.EpochRef, exit ExitRecord) {
	var ids []contracts.WaitID
	for id := range k.waits {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		wait := k.waits[id]
		if wait.caller == epoch {
			delete(k.waits, id)
			continue
		}
		if wait.target == epoch && !wait.exit.Valid {
			wait.exit = exit
		}
	}
}

func consumeEpoch(m *Machine) (uint64, error) {
	if m.nextEpoch == 0 {
		return 0, errors.New("machine epoch space exhausted")
	}
	epoch := m.nextEpoch
	if epoch == math.MaxUint64 {
		m.nextEpoch = 0
	} else {
		m.nextEpoch++
	}
	return epoch, nil
}
