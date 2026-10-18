package simulation

// TODO(migration): remove in Task 22; the live kernel supplies these ports after cutover.
import (
	"log/slog"
	"math"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
	network "github.com/neria-cloud/gosim/internal/net/model"
	"github.com/neria-cloud/gosim/internal/simulation/syscallabi"
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

type legacyNetworkClock struct{}

func (legacyNetworkClock) NowNanos() int64 { return time.Now().UnixNano() }

type legacyNetworkTrace struct{}

func (legacyNetworkTrace) Emit(_ contracts.EpochRef, name string, fields []contracts.Field) {
	if !gosimruntime.TraceNetwork.Enabled() {
		return
	}
	attrs := make([]any, 0, 2*len(fields)+2)
	for _, f := range fields {
		attrs = append(attrs, f.Key, f.Value)
	}
	attrs = append(attrs, "traceKind", "network")
	slog.Info(name, attrs...)
}

type legacyNetworkEvent struct {
	ref      contracts.EpochRef
	timer    *time.Timer
	callback func()
}
type legacyNetworkEvents struct {
	dispatcher syscallabi.Dispatcher
	next       contracts.EventID
	pending    map[contracts.EventID]*legacyNetworkEvent
	live       map[contracts.EpochRef]bool
}

func (e *legacyNetworkEvents) Schedule(ref contracts.EpochRef, at int64, callback func()) (contracts.EventID, error) {
	if !e.live[ref] {
		return 0, contracts.ErrInvalidState
	}
	if e.next == contracts.EventID(math.MaxUint64) {
		panic("legacy network events exhausted")
	}
	e.next++
	id := e.next
	entry := &legacyNetworkEvent{ref: ref, callback: callback}
	e.pending[id] = entry
	// Timers enqueue commands; only the syscall supervisor touches model or event state.
	entry.timer = time.AfterFunc(time.Duration(max(int64(0), at-time.Now().UnixNano())), func() {
		invocation := &syscallabi.Syscall{Trampoline: func(call *syscallabi.Syscall) {
			if event := e.pending[id]; event != nil {
				delete(e.pending, id)
				callback := event.callback
				event.callback = nil
				event.timer = nil
				if e.live[event.ref] {
					callback()
				}
			}
			call.Complete()
		}}
		e.dispatcher.Dispatch(invocation)
	})
	return id, nil
}

func (e *legacyNetworkEvents) Cancel(id contracts.EventID) {
	if event := e.pending[id]; event != nil {
		event.timer.Stop()
		event.callback = nil
		event.timer = nil
		delete(e.pending, id)
	}
}

func (e *legacyNetworkEvents) CancelEpoch(ref contracts.EpochRef) {
	for id, event := range e.pending {
		if event.ref == ref {
			e.Cancel(id)
		}
	}
}

func (s *Simulation) setupNetworkPorts() {
	id, err := gosimruntime.NewRunID()
	if err != nil {
		panic(err)
	}
	s.networkRunID = id
	events := &legacyNetworkEvents{dispatcher: s.dispatcher, pending: make(map[contracts.EventID]*legacyNetworkEvent), live: make(map[contracts.EpochRef]bool)}
	random := gosimruntime.NewDomainRandom(gosimruntime.Seed(), "net", 0)
	s.networkEvents = events
	s.networkPorts = contracts.Ports{Clock: legacyNetworkClock{}, Events: events, Trace: legacyNetworkTrace{}, RandomFor: func(feature contracts.Feature, _ contracts.MachineRef) contracts.Random {
		if feature != contracts.Net {
			panic("wrong random domain")
		}
		return random
	}}
	s.network = network.NewNetwork(s.networkPorts.Clock, events, random, s.networkPorts.Trace)
}
