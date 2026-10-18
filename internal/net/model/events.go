package model

import (
	"math"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
)

type modelTimer struct {
	stack      *Stack
	event      contracts.EventID
	generation uint64
	callback   func()
}

func (s *Stack) after(delay time.Duration, callback func()) *modelTimer {
	t := &modelTimer{stack: s, callback: callback}
	t.Reset(delay)
	return t
}

func (t *modelTimer) Stop() {
	if t.event != 0 {
		t.stack.ports.Events.Cancel(t.event)
		t.event = 0
	}
	if t.generation == math.MaxUint64 {
		panic("network timer generation exhausted")
	}
	t.generation++
}

func (t *modelTimer) Reset(delay time.Duration) {
	t.Stop()
	s := t.stack
	if s.quiesced || s.shutdown {
		return
	}
	generation := t.generation
	at, _ := saturatingArrival(s.ports.Clock.NowNanos(), delay)
	id, err := s.ports.Events.Schedule(s.ref, at, func() {
		if generation != t.generation || s.quiesced || s.shutdown {
			return
		}
		t.event = 0
		t.callback()
	})
	if err != nil {
		panic(err)
	}
	t.event = id
}

type pollers struct {
	sinks []contracts.ReadinessSink
	mask  contracts.Readiness
}

func (p *pollers) add(ref contracts.EpochRef, sink contracts.ReadinessSink) {
	for _, old := range p.sinks {
		if old == sink {
			panic("duplicate network poller")
		}
	}
	p.sinks = append(p.sinks, sink)
	sink.Notify(ref, 0, p.mask)
}

func (p *pollers) remove(sink contracts.ReadinessSink) {
	for i, old := range p.sinks {
		if old == sink {
			copy(p.sinks[i:], p.sinks[i+1:])
			p.sinks[len(p.sinks)-1] = nil
			p.sinks = p.sinks[:len(p.sinks)-1]
			return
		}
	}
}

func (p *pollers) set(ref contracts.EpochRef, read, write, hangup bool) {
	var mask contracts.Readiness
	if read {
		mask |= contracts.ReadReady
	}
	if write {
		mask |= contracts.WriteReady
	}
	if hangup {
		mask |= contracts.Hangup
	}
	if mask == p.mask {
		return
	}
	p.mask = mask
	for _, sink := range p.sinks {
		sink.Notify(ref, 0, mask)
	}
}
func (p *pollers) clear() { clear(p.sinks); p.sinks = nil }
