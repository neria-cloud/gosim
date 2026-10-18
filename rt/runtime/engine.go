package gosimruntime

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"slices"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// ReplayRevision changes whenever scheduling/random derivation semantics change.
const ReplayRevision = "gosim-replay-v2"

type EngineConfig struct {
	Seed           int64
	TimeoutNS      int64
	ReplayRevision string
}

type MachineToken struct{ ID, Epoch uint64 }

var (
	ErrEngineBusy     = errors.New("another simulation is active in this process")
	ErrEngineState    = errors.New("engine is not active or has already run")
	ErrMachineToken   = errors.New("invalid or stale runtime machine token")
	ErrEventExhausted = errors.New("runtime event ID space exhausted")
)

type engineMachine struct {
	token     MachineToken
	machine   *Machine
	published bool
	canceled  bool
	retired   []*goroutine // retained until invocation borrowing has ended
}

type engineEvent struct {
	owner MachineToken
	timer *Timer
}

type randomKey struct {
	domain  string
	machine uint64
}

// Engine owns one scheduler and all native machine allocations in a run.
// Its methods run on the serialized supervisor (or through a native upcall).
type Engine struct {
	timeoutTimer    *Timer
	config          EngineConfig
	scheduler       *scheduler
	machines        map[uint64]*engineMachine
	lastEpoch       map[uint64]uint64
	events          map[uint64]engineEvent
	nextEvent       uint64
	random          map[randomKey]*Random
	supervisor      *Machine
	supervisorToken RaceToken
	running         bool
	cleaning        bool
	ran             bool
	closed          bool
	result          error
}

func NewEngine(config EngineConfig) (*Engine, error) {
	if config.TimeoutNS <= 0 || config.ReplayRevision != ReplayRevision {
		return nil, fmt.Errorf("invalid engine timeout or replay revision")
	}
	e := &Engine{
		config: config, machines: make(map[uint64]*engineMachine), lastEpoch: make(map[uint64]uint64), supervisorToken: MakeRaceToken(),
		events: make(map[uint64]engineEvent), random: make(map[randomKey]*Random), nextEvent: 1,
	}
	e.scheduler = newScheduler(config.Seed, makeLogger(io.Discard, slog.LevelInfo), nil, nil)
	e.scheduler.engine = e
	e.scheduler.fastrander.state = NewDomainRandom(config.Seed, "scheduler", 0).state
	e.random[randomKey{domain: "scheduler"}] = (*Random)(&e.scheduler.fastrander)
	return e, nil
}

// onSupervisor does not yield to another application goroutine. With the race
// detector it transfers scheduler mutations to the native scheduler stack.
//
//go:norace
func (e *Engine) onSupervisor(f func()) error {
	if !e.running || e.closed || gs.get() != e.scheduler {
		return ErrEngineState
	}
	var failed bool
	checked := func() {
		defer func() {
			if failure := recover(); failure != nil {
				failed = true
				e.scheduler.abortErr = &AbortError{Diagnostic: abi.Diagnostic{Kind: "InvariantFailure", Message: fmt.Sprint(failure)}}
			}
		}()
		f()
	}
	if g := getg(); g != nil && !e.scheduler.inupcall {
		g.upcall(checked)
		if failed {
			g.park(false)
		}
	} else {
		checked()
	}
	return nil
}

//go:norace
func (e *Engine) PrepareMachine(id, epoch uint64, label string) (MachineToken, error) {
	var token MachineToken
	var result error
	err := e.onSupervisor(func() {
		if e.cleaning {
			result = ErrEngineState
			return
		}
		if id == 0 || epoch == 0 || epoch <= e.lastEpoch[id] || e.machines[id] != nil {
			result = ErrMachineToken
			return
		}
		token = MachineToken{ID: id, Epoch: epoch}
		e.lastEpoch[id] = epoch
		// Allocation alone neither initializes globals nor starts a coroutine.
		m := newMachine(label)
		m.token = token
		e.machines[id] = &engineMachine{token: token, machine: m}
	})
	return token, errors.Join(err, result)
}

//go:norace
func (e *Engine) PublishMachine(token MachineToken, boot func()) {
	var invalid bool
	entryToken := MakeRaceToken()
	entryToken.Release()
	err := e.onSupervisor(func() {
		m := e.lookup(token)
		if m == nil || m.published || m.canceled || boot == nil || e.cleaning {
			invalid = true
			return
		}
		m.published = true
		// Starting the coroutine parks before its entry function; no boot/init
		// executes within this publication barrier.
		e.scheduler.Go(boot, m.machine, entryToken)
	})
	if err != nil || invalid {
		e.invariant("publish invalid runtime machine")
	}
}

func (e *Engine) lookup(token MachineToken) *engineMachine {
	m := e.machines[token.ID]
	if m == nil || m.token != token {
		return nil
	}
	return m
}

//go:norace
func (e *Engine) AbortMachine(token MachineToken) {
	var published bool
	err := e.onSupervisor(func() {
		if m := e.lookup(token); m != nil {
			if m.published {
				published = true
				return
			}
			m.machine.globals = nil
			delete(e.machines, token.ID)
		}
	})
	if err != nil || published {
		e.invariant("abort after runtime publication or outside active engine")
	}
}

//go:norace
func (e *Engine) CancelMachine(token MachineToken) {
	var self bool
	err := e.onSupervisor(func() {
		m := e.lookup(token)
		if m == nil || m.canceled {
			return
		}
		if g := getg(); g != nil && g.machine == m.machine {
			self = true
			return
		}
		e.cancel(m)
	})
	if err != nil || self {
		e.invariant("runtime cancellation must run on the surviving supervisor")
	}
}

func (e *Engine) cancel(m *engineMachine) {
	m.canceled = true // admission closes before any callback/coroutine is removed
	var eventIDs []uint64
	for id, event := range e.events {
		if event.owner == m.token {
			eventIDs = append(eventIDs, id)
		}
	}
	slices.Sort(eventIDs)
	for _, id := range eventIDs {
		e.events[id].timer.Stop()
		delete(e.events, id)
	}
	e.scheduler.clock.removemachine(m.machine)
	var doomed []*goroutine
	for _, g := range e.scheduler.goroutines {
		if g.machine == m.machine {
			doomed = append(doomed, g)
		}
	}
	slices.SortFunc(doomed, func(a, b *goroutine) int { return a.ID - b.ID })
	for _, g := range doomed {
		g.abort() // finishes coroutines without unwinding application defers
		e.scheduler.removeGoroutine(g)
		m.retired = append(m.retired, g)
	}
	pruneSemaphores()
	// Globals remain borrowed until the kernel has drained invocations and calls Dispose.
}

//go:norace
func (e *Engine) DisposeMachine(token MachineToken) {
	var live bool
	err := e.onSupervisor(func() {
		if m := e.lookup(token); m != nil {
			if m.published && !m.canceled {
				live = true
				return
			}
			m.machine.globals = nil
			for _, g := range m.retired {
				freeGoroutine(g)
			}
			m.retired = nil
			delete(e.machines, token.ID)
		}
	})
	if err != nil || live {
		e.invariant("dispose before runtime cancellation")
	}
}

//go:norace
func (e *Engine) Schedule(token MachineToken, when int64, callback func()) (uint64, error) {
	var id uint64
	var result error
	err := e.onSupervisor(func() {
		m := e.lookup(token)
		if m == nil || !m.published || m.canceled || callback == nil {
			result = ErrMachineToken
			return
		}
		if e.nextEvent == 0 {
			result = ErrEventExhausted
			return
		}
		id = e.nextEvent
		e.nextEvent++
		timer := NewTimer(func(*Timer) {
			delete(e.events, id)
			if current := e.lookup(token); current != nil && current.published && !current.canceled {
				e.supervisorToken.Acquire()
				defer e.supervisorToken.Release()
				previous := globalPtr.get()
				globalPtr.set(e.supervisor.globals)
				defer globalPtr.set(previous)
				callback()
			}
		}, nil, m.machine, when)
		e.events[id] = engineEvent{owner: token, timer: timer}
	})
	return id, errors.Join(err, result)
}

//go:norace
func (e *Engine) CancelEvent(id uint64) {
	if err := e.onSupervisor(func() {
		if event, ok := e.events[id]; ok {
			event.timer.Stop()
			delete(e.events, id)
		}
	}); err != nil {
		e.invariant("cancel event outside active engine")
	}
}

type AbortError struct{ Diagnostic abi.Diagnostic }

func (err *AbortError) Error() string {
	d := err.Diagnostic
	return fmt.Sprintf("%s: feature=%s operation=%s machine=%d epoch=%d pc=%#x: %s", d.Kind, d.Feature, d.Operation, d.MachineID, d.Epoch, d.PC, d.Message)
}

func (e *Engine) invariant(message string) {
	e.Abort(abi.Diagnostic{Kind: "InvariantFailure", Message: message})
}

// Abort records failure before parking the caller. It is not a panic and
// application recover/defer cannot turn it into a successful run.
//
//go:norace
func (e *Engine) Abort(diagnostic abi.Diagnostic) {
	err := &AbortError{Diagnostic: diagnostic}
	if !e.running || gs.get() != e.scheduler {
		e.result = err
		return
	}
	_ = e.onSupervisor(func() {
		if e.cleaning {
			e.scheduler.abortErr = errors.Join(e.scheduler.abortErr, err)
		} else if e.scheduler.abortErr == nil {
			e.scheduler.abortErr = err
		}
	})
	if g := getg(); g != nil && !e.cleaning {
		g.park(false)
	}
}

// RandomFor returns the same owned stream for a domain/logical machine for the
// whole run. Epoch and process run identity never affect its state.
//
//go:norace
func (e *Engine) RandomFor(domain string, machineID uint64) *Random {
	if domain != "scheduler" && domain != "application" && domain != "fs" && domain != "net" {
		e.invariant("unknown random stream domain")
		return nil
	}
	var stream *Random
	if err := e.onSupervisor(func() {
		key := randomKey{domain: domain, machine: machineID}
		stream = e.random[key]
		if stream == nil {
			stream = NewDomainRandom(e.config.Seed, domain, machineID)
			e.random[key] = stream
		}
	}); err != nil {
		e.invariant("random stream requested outside active engine")
	}
	return stream
}

func (e *Engine) Run(supervise func()) (result error) {
	// TODO(not planned): concurrent independent simulation engines in one process.
	release, err := trySetGS(e.scheduler)
	if err != nil {
		return err
	}
	defer release()
	if e.closed || e.ran || e.result != nil || supervise == nil {
		return errors.Join(ErrEngineState, e.result)
	}
	e.ran, e.running = true, true
	defer func() {
		if failure := recover(); failure != nil {
			result = &AbortError{Diagnostic: abi.Diagnostic{Kind: "InvariantFailure", Message: fmt.Sprint(failure)}}
		}
		e.running = false
		e.result = result
	}()
	if !runtimeInitialized {
		prepareGlobals() // layouts only; the published boot owns application init
		runtimeInitialized = true
	}
	InitSteps()
	e.supervisor = newMachine("supervisor")
	// Publish values captured after construction at the actual run boundary.
	e.scheduler.globaltoken.Release()
	e.scheduler.Go(supervise, e.supervisor, e.scheduler.globaltoken)
	now := e.scheduler.clock.now
	if e.config.TimeoutNS > math.MaxInt64-now {
		e.scheduler.cleanup()
		return fmt.Errorf("simulation timeout overflows virtual time")
	}
	e.timeoutTimer = NewTimer(func(*Timer) { e.scheduler.abortErr = ErrTimeout }, nil, e.supervisor, now+e.config.TimeoutNS)
	return e.scheduler.Run()
}

func (e *Engine) Close() error {
	if e.closed {
		return e.result
	}
	if e.running {
		return fmt.Errorf("close engine after supervisor cleanup and Run return")
	}
	// Run has canceled coroutines and leases can no longer be used. Do not run
	// application code or re-enter a scheduler while releasing remaining state.
	var ids []uint64
	for id := range e.machines {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		m := e.machines[id]
		m.machine.globals = nil
		for _, g := range m.retired {
			freeGoroutine(g)
		}
		m.retired = nil
	}
	if e.supervisor != nil {
		e.supervisor.globals = nil
	}
	e.machines, e.events, e.random, e.lastEpoch = nil, nil, nil, nil
	e.scheduler.clock.heap = newTimerHeap()
	e.timeoutTimer = nil
	e.closed = true
	return e.result
}

// SetTimeout replaces the run deadline relative to current virtual time. It
// mutates only the supervisor timer and never consults or creates a host timer.
//
//go:norace
func (e *Engine) SetTimeout(ns int64) error {
	if ns <= 0 {
		return fmt.Errorf("simulation timeout must be positive")
	}
	var result error
	err := e.onSupervisor(func() {
		now := e.scheduler.clock.now
		if e.timeoutTimer == nil || ns > math.MaxInt64-now {
			result = fmt.Errorf("simulation timeout overflows virtual time")
			return
		}
		e.timeoutTimer.Reset(now + ns)
		e.config.TimeoutNS = ns
	})
	return errors.Join(err, result)
}
