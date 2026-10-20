package gosimruntime

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/neria-cloud/gosim/internal/hostabi"
)

const schedulerInitialCapacity = 1024

type constErr struct {
	error
}

func makeConstErr(err error) error {
	return constErr{error: err}
}

var (
	ErrPaniced      = makeConstErr(errors.New("paniced"))
	ErrAborted      = makeConstErr(errors.New("aborted"))
	ErrMainReturned = makeConstErr(errors.New("main returned"))
	ErrTimeout      = makeConstErr(errors.New("timeout"))
	ErrTestFailed   = makeConstErr(errors.New("test failed"))
)

func getEntryFunc(fun func()) string {
	return runtime.FuncForPC(reflect.ValueOf(fun).Pointer()).Name()
}

var aslrBaseAddr = uintptr(reflect.ValueOf(reflect.ValueOf).UnsafePointer())

// relativeFuncAddr returns the address of a function ignoring address space
// layout randomization. This makes function pointers comparable across
// processes to check determinism.
func relativeFuncAddr(fun func()) uintptr {
	return uintptr(reflect.ValueOf(fun).UnsafePointer()) - aslrBaseAddr
}

var (
	insideGosimRun atomic.Bool
	gs             atomicValue[*scheduler]
)

func setgs(newgs *scheduler) func() {
	release, err := trySetGS(newgs)
	if err != nil {
		panic(err)
	}
	return release
}

func trySetGS(newgs *scheduler) (func(), error) {
	if !insideGosimRun.CompareAndSwap(false, true) {
		return nil, ErrEngineBusy
	}
	gs.set(newgs)
	if newgs.current.get() != nil {
		panic("when setting gs expecting nil current")
	}
	globalPtr.set(nil)

	return func() {
		gs.set(nil)
		globalPtr.set(nil)
		if !insideGosimRun.CompareAndSwap(true, false) {
			panic("gosimruntime already stopped!!!???")
		}
	}, nil
}

func getg() *goroutine {
	return gs.get().current.get()
}

type Machine struct {
	label   string
	globals unsafe.Pointer
	token   MachineToken
}

func newMachine(label string) *Machine {
	return &Machine{
		label:   label,
		globals: allocGlobals(),
	}
}

type scheduler struct {
	seed            int64
	engine          *Engine
	transitionDepth int

	goroutines      intrusiveList[*goroutine]
	runnable        intrusiveList[*goroutine]
	nextGoroutineID int

	goroutinesById map[int]*goroutine

	envs []string

	current atomicValue[*goroutine]

	checksummer *checksummer

	fastrander fastrander

	clock *clock

	nextEventID int

	logger logger

	abortErr error

	// race token given to background goroutines so they can read gs fields
	globaltoken RaceToken

	inupcall bool
}

func newScheduler(seed int64, logger logger, checksummer *checksummer, extraenv []string) *scheduler {
	clock := newClock()

	rand := mathrand.New(mathrand.NewSource(seed))

	s := &scheduler{
		seed:            seed,
		goroutines:      make([]*goroutine, 0, schedulerInitialCapacity),
		runnable:        make([]*goroutine, 0, schedulerInitialCapacity),
		nextGoroutineID: 1,
		fastrander:      fastrander{state: rand.Uint64()},
		goroutinesById:  make(map[int]*goroutine),
		clock:           clock,
		nextEventID:     1,
		logger:          logger,
		checksummer:     checksummer,
		globaltoken:     MakeRaceToken(),
		envs: append(extraenv,
			"GOSIM_LOG_LEVEL="+logger.level.String(),
		),
	}
	s.globaltoken.Release()

	return s
}

type intrusiveList[A any] []A

type intrusiveIndex struct{ idx int }

func (l *intrusiveList[A]) add(elem A, idxFunc func(elem A) *intrusiveIndex) {
	oldIdx := idxFunc(elem).idx
	if oldIdx != -1 {
		panic(elem)
	}
	idxFunc(elem).idx = len(*l)
	*l = append(*l, elem)
}

func (l *intrusiveList[A]) remove(elem A, idxFunc func(elem A) *intrusiveIndex) {
	oldIdx := idxFunc(elem).idx
	if oldIdx == -1 {
		panic(elem)
	}
	n := len(*l)
	replacement := (*l)[n-1]
	(*l)[oldIdx] = replacement
	idxFunc(replacement).idx = oldIdx
	*l = (*l)[:n-1]
	idxFunc(elem).idx = -1
}

func (s *scheduler) addGoroutine(g *goroutine) {
	s.goroutines.add(g, (*goroutine).allIdxPtr)
	s.goroutinesById[g.ID] = g
	s.addRunnableGoroutine(g)
}

func (s *scheduler) removeGoroutine(g *goroutine) {
	s.goroutines.remove(g, (*goroutine).allIdxPtr)
	delete(s.goroutinesById, g.ID)
	if g.runnableIdx.idx != -1 {
		s.removeRunnableGoroutine(g)
	}
}

func (s *scheduler) addRunnableGoroutine(g *goroutine) {
	s.runnable.add(g, (*goroutine).runnableIdxPtr)
}

func (s *scheduler) removeRunnableGoroutine(g *goroutine) {
	s.runnable.remove(g, (*goroutine).runnableIdxPtr)
}

func (s *scheduler) pickRandomOtherGoroutine(g *goroutine) *goroutine {
	if len(s.goroutines) == 1 {
		return nil
	}
	idx := fastrandn(uint32(len(s.goroutines) - 1))
	if s.goroutines[idx] == g {
		idx++
	}
	return s.goroutines[idx]
}

func (s *scheduler) getNextGoroutineID() int {
	id := s.nextGoroutineID
	s.nextGoroutineID++
	return id
}

func (s *scheduler) Go(fun func(), machine *Machine, token RaceToken) *goroutine {
	id := s.getNextGoroutineID()

	eventID := NextEventID()

	if !token.Empty() {
		// for getEntryFunc
		token.Acquire()
	}
	if s.logger.slog.Enabled(context.TODO(), slog.LevelDebug) {
		s.logger.slog.Log(context.TODO(), slog.LevelDebug, "spawning goroutine", "eventid", eventID, "childgoroutine", id, "entry", getEntryFunc(fun))
	}

	// XXX: log/trace happens-before spawn, happens-after start?
	newg := newGoroutine(id, getg(), eventID, fun, machine, token)
	s.addGoroutine(newg)

	return newg
}

func (s *scheduler) Run() (result error) {
	defer func() {
		previous := s.abortErr
		s.cleanup()
		if result == previous {
			result = s.abortErr
		} else if s.abortErr != previous {
			result = errors.Join(result, s.abortErr)
		}
	}()
	// TODO: add a wall-clock timeout for running any given step. If the step
	// hangs for too long, it probably is doing something weird (using
	// uninstrumented sync or some spin.) Call out the stuck goroutine as the
	// problem.

	// TODO: move scheduler deterministic seed/state initialization here?

	for (len(s.goroutines) > 0 || s.engine != nil && len(s.engine.events) > 0) && s.abortErr == nil {
		// XXX: can we make the scheduler itself run in the goroutines? so we
		// don't have to ping-pong another goroutine switch

		for {
			handler, ok := s.clock.maybefire()
			if !ok {
				break
			}
			if handler != nil {
				handler.handler(handler)
				// s.firetimer(handler)
			}
			if s.abortErr != nil {
				break
			}
		}
		if s.abortErr != nil || (len(s.goroutines) == 0 && (s.engine == nil || len(s.engine.events) == 0)) {
			break
		}

		if len(s.runnable) == 0 && s.clock.anyWaiting() {
			s.clock.doadvance()
			continue
		}

		if len(s.runnable) == 0 {
			buf := make([]byte, 256*1024)
			buf = buf[:runtime.Stack(buf, true)]
			return fmt.Errorf("all goroutines blocked\n\n%s", string(buf))
		}

		pick := s.runnable[s.fastrander.fastrandn(uint32(len(s.runnable)))]

		if s.checksummer != nil {
			s.checksummer.recordIntInt(checksumKeyRunPick, uint64(pick.ID), uint64(s.fastrander.state)) // #nosec G115 -- goroutine IDs are nonnegative.
		}
		s.current.set(pick)
		pick.step()
		s.current.set(nil)
		if s.checksummer != nil {
			var flags uint64
			if pick.finished {
				flags |= 1
			}
			if pick.waiting {
				flags |= 1 << 1
			}
			s.checksummer.recordIntInt(checksumKeyRunResult, flags, s.fastrander.state)
		}
		if pick.finished {
			s.removeGoroutine(pick)
			freeGoroutine(pick)
		}
	}

	// TODO: Test abortErr?
	return s.abortErr
}

func (s *scheduler) cleanup() {
	if s.engine != nil {
		s.engine.cleaning = true
		defer func() { s.engine.cleaning = false }()
	}
	// Also run on deadlock/error returns, which must not leave parked coroutines.
	doomed := slices.Clone(s.goroutines)
	slices.SortFunc(doomed, func(a, b *goroutine) int { return a.ID - b.ID })
	var supervisor *goroutine
	for _, g := range doomed {
		if s.engine != nil && g.machine == s.engine.supervisor {
			supervisor = g
			continue
		}
		g.abort()
		s.removeGoroutine(g)
		if s.engine != nil {
			if m := s.engine.lookup(g.machine.token); m != nil {
				m.retired = append(m.retired, g)
				continue
			}
		}
		freeGoroutine(g)
	}
	if supervisor != nil {
		// Only trusted harness defers may run during abort. Application machine
		// coroutines above are canceled without executing any of their defers.
		supervisor.finishSupervisor()
		s.removeGoroutine(supervisor)
		freeGoroutine(supervisor)
	}
	s.goroutines, s.runnable = nil, nil
	s.clock.heap = newTimerHeap()
	pruneSemaphores()
}

var runtimeInitialized bool

func Seed() int64 { return gs.get().seed }

type corotask int

const (
	corotaskNone corotask = iota
	corotaskAbort
	corotaskStep
	corotaskBacktrace
	corotaskCleanup
)

type goroutine struct {
	ID          int
	allIdx      intrusiveIndex // idx in scheduler.goroutines, or -1
	runnableIdx intrusiveIndex // idx in scheduler.runnable, or -1

	spawnEventID  int
	machine       *Machine
	parent        *goroutine
	profileLabels string
	procPinDepth  int

	finished bool

	coro hostabi.Upcallcoro

	corotask corotask

	backtracePc []uintptr
	backtraceN  int

	parksynctoken RaceToken
	shouldacquire bool
	shouldrelease *racesynchelper

	selected *sudog
	waiters  *sudog // linked-list

	// true if called yield(true) and not yet continued
	waiting bool
	// true if explicitly paused
	paused bool

	parkWait           bool
	parkPaniced        bool
	parkPanicTraceback []byte
	parkPanicValue     string

	entryFun   func()
	entryToken RaceToken
}

func newGoroutine(id int, parent *goroutine, spawnEventID int, fun func(), machine *Machine, token RaceToken) *goroutine {
	gs := gs.get()

	g := allocGoroutine()
	g.ID = id
	g.parent = parent
	if parent != nil && parent.machine == machine {
		g.profileLabels = parent.profileLabels
	}
	g.spawnEventID = spawnEventID
	g.allIdx.idx = -1
	g.runnableIdx.idx = -1
	g.machine = machine
	g.parksynctoken = MakeRaceToken()

	if gs.checksummer != nil {
		// #nosec G115 -- goroutine IDs start at one and increase monotonically.
		gs.checksummer.recordIntInt(checksumKeyCreating, uint64(g.ID), uint64(relativeFuncAddr(fun))) // XXX: test this pointer
	}

	prev := getg()
	gs.current.set(g)
	g.entryFun = fun
	g.entryToken = token
	hostabi.RaceDisable() // make sure no happens-before to the scheduler
	g.coro.Start(goroutineEntrypoint)
	hostabi.RaceEnable()
	gs.current.set(prev)

	return g
}

var goroutinePool []*goroutine

func allocGoroutine() *goroutine {
	if n := len(goroutinePool); n > 0 {
		g := goroutinePool[n-1]
		goroutinePool = goroutinePool[:n-1]
		return g
	}
	return &goroutine{}
}

// XXX: if ANYONE still refers to this goroutine we will be in a world of
// pain... eg. select or timer of an aborted goroutine???????????
func freeGoroutine(g *goroutine) {
	g.ID = 0
	g.allIdx.idx = -1
	g.runnableIdx.idx = -1
	g.spawnEventID = 0
	g.machine = nil
	g.parent = nil
	g.profileLabels = ""
	g.procPinDepth = 0
	g.finished = false
	g.coro = hostabi.Upcallcoro{}
	g.corotask = corotaskNone
	g.parksynctoken = RaceToken{}
	g.shouldacquire = false
	g.shouldrelease = nil
	g.selected = nil
	g.waiters = nil
	g.waiting = false
	g.paused = false
	g.parkWait = false
	g.parkPaniced = false
	g.parkPanicTraceback = nil
	g.parkPanicValue = ""
	g.entryFun = nil
	g.entryToken = RaceToken{}
	goroutinePool = append(goroutinePool, g)
}

func (g *goroutine) allIdxPtr() *intrusiveIndex {
	return &g.allIdx
}

func (g *goroutine) runnableIdxPtr() *intrusiveIndex {
	return &g.runnableIdx
}

func (g *goroutine) setWaiting(waiting bool) {
	gs := gs.get()
	g.waiting = waiting
	if g.runnableIdx.idx == -1 && !(g.waiting || g.paused) {
		gs.addRunnableGoroutine(g)
	} else if g.runnableIdx.idx != -1 && (g.waiting || g.paused) {
		gs.removeRunnableGoroutine(g)
	}
}

func (g *goroutine) setPaused(paused bool) {
	gs := gs.get()
	g.paused = paused
	if g.runnableIdx.idx == -1 && !(g.waiting || g.paused) {
		gs.addRunnableGoroutine(g)
	} else if g.runnableIdx.idx != -1 && (g.waiting || g.paused) {
		gs.removeRunnableGoroutine(g)
	}
}

// TODO: should this be norace? think about it
//
//go:norace
func (g *goroutine) allocWaiter() *sudog {
	sg := allocSudog()
	sg.goroutine = g
	sg.waitersNext = g.waiters
	g.waiters = sg
	return sg
}

// TODO: should this be norace? think about it
//
//go:norace
func (g *goroutine) releaseWaiters() {
	g.selected = nil
	next := g.waiters
	g.waiters = nil
	for next != nil {
		cur := next
		cur.goroutine = nil
		cur.item = nil
		next = cur.waitersNext
		cur.waitersNext = nil
		freeSudog(cur)
	}
}

const debugUpcall = true

//go:norace
func (g *goroutine) upcall(f func()) {
	hostabi.RaceDisable()
	if debugUpcall {
		if gs.get().inupcall {
			panic("already in upcall")
		}
		gs.get().inupcall = true
	}
	g.coro.Upcall(f)
	if debugUpcall {
		gs.get().inupcall = false
	}
	hostabi.RaceEnable()
}

func (g *goroutine) step() {
	globalPtr.set(g.machine.globals)

	if inControlledNondeterminism() {
		panic("help")
	}

	g.corotask = corotaskStep
	g.coro.Next()
	if g.parkWait {
		g.setWaiting(true)
	}
	if g.parkPaniced {
		if gs.get().abortErr == nil {
			gs.get().abortErr = ErrPaniced
		}
		gs.get().logger.slog.Error("uncaught panic",
			"traceback", strings.Split(strings.ReplaceAll(string(g.parkPanicTraceback), "\t", "  "), "\n"),
			"panic", g.parkPanicValue)
	}
}

// abort stops the goroutine.
// this is a dangerous call, make sure that noone will refer to the goroutine later.
func (g *goroutine) abort() {
	gs := gs.get()

	// XXX: trace?
	if getg() == g {
		panic("aborting self")
	}

	// if currently waiting, unhook from queues. for crash support. queue code
	// better be resilient...
	if g.waiters != nil && g.selected == nil {
		for sg := g.waiters; sg != nil; sg = sg.waitersNext {
			sg.queue.remove(sg)
		}
	}
	g.releaseWaiters()

	// XXX: if g is in any kind of select operation, carefully unwind that??? so
	// far, not necessary because we abort entire machines at a time. (though
	// what about cross-machine selects??? help.)

	prev := getg()
	gs.current.set(g)
	g.corotask = corotaskAbort
	g.coro.Next()
	gs.current.set(prev)

	if !g.finished {
		panic("expected finished")
	}
}

func (g *goroutine) finishSupervisor() {
	s := gs.get()
	if s.engine == nil || g.machine != s.engine.supervisor || getg() == g {
		panic("invalid supervisor cleanup")
	}
	if g.waiters != nil && g.selected == nil {
		for sg := g.waiters; sg != nil; sg = sg.waitersNext {
			sg.queue.remove(sg)
		}
	}
	g.releaseWaiters()
	previous, globals := getg(), globalPtr.get()
	s.current.set(g)
	globalPtr.set(g.machine.globals)
	defer func() { s.current.set(previous); globalPtr.set(globals) }()
	g.corotask = corotaskCleanup
	g.coro.Next()
	if !g.finished {
		panic("supervisor cleanup yielded")
	}
}

func (g *goroutine) backtrace(pc []uintptr) int {
	if getg() == g {
		panic("backtrace self")
	}

	g.backtracePc = pc
	g.corotask = corotaskBacktrace
	g.coro.Next()
	g.backtracePc = nil

	n := g.backtraceN
	g.backtraceN = 0

	return n
}

// Marked norace to read entryToken and entryFun.
//
//go:norace
func goroutineEntrypoint() {
	g := getg()
	// no need to allocate this entrypoint
	if !g.entryToken.Empty() {
		g.entryToken.Acquire()
	}

	g.park(false)
	defer g.exitpoint()
	g.entryFun()
}

// exitpoint is a helper for entrypoint that stores any exit panic value
// and marks the coroutine as finished.
//
// It cannot be inlined because go:norace does not apply internally.
//
// The g.finished = true must be in a recover call because the goroutine
// might exit by calling runtime.Goexit()
//
//go:norace
func (g *goroutine) exitpoint() {
	if recovered := recover(); recovered != nil {
		g.parkPaniced = true
		traceback := make([]byte, 32*1024)
		traceback = traceback[:runtime.Stack(traceback, false)]
		// XXX: necessary?
		g.parkPanicTraceback = NoraceCopy(traceback)
		g.parkPanicValue = fmt.Sprint(recovered)
	}
	g.finished = true
	g.releaseSupervisor()
	g.coro.Finish()
}

//go:norace
func (g *goroutine) park(wait bool) {
	if inControlledNondeterminism() {
		if wait {
			panic("help")
		}
		return
	}

	g.parkWait = wait

	// inform scheduler we're paused
	// wait for us to resume
	for {
		g.releaseSupervisor()
		if hostabi.RaceEnabled {
			if wait {
				g.parksynctoken.Release()
			}
			hostabi.RaceDisable()
		}

		g.coro.Yield()

		if hostabi.RaceEnabled {
			hostabi.RaceEnable()
			if wait {
				if g.shouldacquire {
					hostabi.RaceAcquire(unsafe.Pointer(&g.shouldacquire))
					g.shouldacquire = false
				}
				if g.shouldrelease != nil {
					addr := g.shouldrelease
					hostabi.RaceRelease(unsafe.Pointer(addr))
					addr.pending = nil
					g.shouldrelease = nil
				}
			}
		}
		g.acquireSupervisor()

		switch g.corotask {
		case corotaskCleanup:
			runtime.Goexit() // trusted supervisor cleanup defers, then exitpoint
		case corotaskAbort:
			g.finished = true
			g.coro.Finish()
		case corotaskStep:
			return
		case corotaskBacktrace:
			g.backtraceN = runtime.Callers(1, g.backtracePc)
		default:
			panic(g.corotask)
		}
		g.corotask = corotaskNone
	}
}

func (g *goroutine) yield() {
	checkTransition()
	// TODO: optimize by fast-pathing here to stay on the same goroutine before
	// hitting the expensive coroutine switching in park?
	g.park(false)
}

func (g *goroutine) wait() {
	checkTransition()
	g.park(true)
}

// Go starts a new goroutine.
//
// Marked go:norace to let upcall access g.
//
//go:norace
func Go(fun func()) {
	g := getg()
	g.yield()

	token := MakeRaceToken()
	token.Release()
	g.upcall(func() {
		gs.get().Go(fun, g.machine, token)
	})
}

//go:norace
func GoWithMachine(fun func(), machine *Machine) {
	g := getg()
	g.yield()

	token := MakeRaceToken()
	token.Release()
	g.upcall(func() {
		gs.get().Go(fun, machine, token)
	})
}

//go:norace
func GoFromTimerHandler(fun func(), machine *Machine) {
	if getg() != nil {
		panic("must be called without g")
	}

	gs.get().Go(fun, machine, gs.get().globaltoken)
}

// Marked go:norace to return result.
//
//go:norace
func PickRandomOtherGoroutine() (goroutineID int) {
	var result int
	g := getg()
	g.upcall(func() {
		result = gs.get().pickRandomOtherGoroutine(g).ID
	})
	return result
}

func PauseGoroutine(goroutineID int) {
	getg().upcall(func() {
		gs.get().goroutinesById[goroutineID].setPaused(true)
	})
}

func ResumeGoroutine(goroutineID int) {
	getg().upcall(func() {
		gs.get().goroutinesById[goroutineID].setPaused(false)
	})
}

func StopMachine(machine *Machine) {
	// TODO: janky method, expose different API?
	getg().upcall(func() {
		gs := gs.get()

		// XXX: why are we doing this list copying game? trying to not lose goroutines
		// if they get shuffled...?
		var toAbort []*goroutine
		// XXX: inefficient?
		for _, g := range gs.goroutines {
			if g.machine == machine {
				toAbort = append(toAbort, g)
			}
		}
		for _, g := range toAbort {
			g.abort()
			gs.removeGoroutine(g)
			freeGoroutine(g)
		}
		// unhook timer events
		// XXX: inefficient:
		gs.clock.removemachine(machine)

		// XXX: poison machine for future use? set globals to nil?
		// XXX: could later also reuse globals struct (after zeroing...) (but maybe not in super paranoid mode)
		machine.globals = nil

		// XXX: mark machine as bad for higher-level errors?
	})
}

func IsSim() bool {
	// TODO: test this doesn't race?
	return gs.get() != nil
}

func Yield() {
	getg().yield()
}

func SetFinalizer(obj any, finalizer any) {
	// never do anything, hopefully no problems...

	// TODO: maybe could have deterministic GC if we disable automatic GC and
	// invoke it ourselves?
}

func SetAbortError(err error) {
	if _, ok := err.(constErr); !ok {
		panic("SetAbortErr must be called with one of the predefined errors in this package")
	}

	getg().upcall(func() {
		gs := gs.get()
		if gs.abortErr == nil {
			gs.abortErr = err
		}
	})

	// call park so this goroutine can check abortErr and exit
	getg().park(false)
}

const GOOS = "linux"

// Marked go:norace to read ID
//
//go:norace
func GetGoroutine() int {
	return getg().ID
}

// Marked go:norace to read machine
//
//go:norace
func CurrentMachine() *Machine {
	return getg().machine
}

// Marked go:norace to read and write nextEventID
//
//go:norace
func NextEventID() int {
	gs := gs.get()
	id := gs.nextEventID
	gs.nextEventID++
	return id
}

var logBuf []byte

//go:norace
func WriteLog(b []byte) {
	logBuf = NoraceAppend(logBuf[:0], b...)
	getg().upcall(func() {
		_, _ = gs.get().logger.out.Write(logBuf)
	})
}

//go:norace
func Envs() []string {
	return NoraceCopy(gs.get().envs)
}

//go:norace
func NewMachine(label string) *Machine {
	var result *Machine
	getg().upcall(func() {
		result = newMachine(label)
	})
	return result
}

var controlledNondeterminismDepth int

func inControlledNondeterminism() bool {
	return controlledNondeterminismDepth > 0
}

// BeginControlledNondeterminism starts a section of code that is expected to
// behave non-deterministically inside the section but that will not show that
// outside the section.
//
// Calls to random in the section always return 0. Any attempt to pause the
// current thread (eg. mutex.Lock) will panic.
//
// Controlled nondeterminism is useful to cache work that can be shared between
// different machines that is expensive to recompute and complicated to
// precompute once before machines start. One use case is the JSON package's
// type cache, see the translate code that inserts calls.
//
//go:norace
func BeginControlledNondeterminism() {
	controlledNondeterminismDepth++
}

//go:norace
func EndControlledNondeterminism() {
	controlledNondeterminismDepth--
}

var globalCounter int

//go:norace
func Nondeterministic() int {
	globalCounter++
	return globalCounter
}

// TODO: expose something like func AssertAllDone() which checks there are no
// other goroutines, no registered timers, ...?

var (
	stepCounter    int
	stepBreakpoint int
)

var stepBreakpointFlag = flag.Int("step-breakpoint", 0, "")

//go:norace
func InitSteps() {
	stepCounter = 0
	stepBreakpoint = *stepBreakpointFlag
}

//go:norace
func Step() int {
	stepCounter++
	if stepCounter == stepBreakpoint {
		runtime.Breakpoint()
	}
	return stepCounter
}

func GetStep() int {
	return stepCounter
}

// GetStacktraceFor captures a stacktrace for any goroutine. It switchers, if
// necessary, to the requested goroutine and captures its stack with
// runtime.Callers.
//
// TODO: is this necessary? maybe we make goroutines capture their stacks
// when doing invoke?
//
//go:norace
func GetStacktraceFor(goroutine int, stack []uintptr) int {
	g := getg()
	if g.ID == goroutine {
		return runtime.Callers(1, stack)
	}

	var n int
	g.upcall(func() {
		gs := gs.get()
		g := gs.goroutinesById[goroutine]
		if g == nil {
			// TODO: return 0 or -1 instead?
			panic("no such goroutine")
		}
		n = g.backtrace(stack)
	})
	return n
}
