package model

import (
	"math"

	"github.com/neria-cloud/gosim/internal/contracts"
)

type testEvent struct {
	ref contracts.EpochRef
	at  int64
	fn  func()
}
type testEvents struct {
	now     int64
	next    contracts.EventID
	pending map[contracts.EventID]testEvent
}

func newTestEvents() *testEvents      { return &testEvents{pending: make(map[contracts.EventID]testEvent)} }
func (e *testEvents) NowNanos() int64 { return e.now }
func (e *testEvents) Schedule(ref contracts.EpochRef, at int64, fn func()) (contracts.EventID, error) {
	e.next++
	e.pending[e.next] = testEvent{ref, at, fn}
	return e.next, nil
}
func (e *testEvents) Cancel(id contracts.EventID) { delete(e.pending, id) }
func (e *testEvents) CancelEpoch(ref contracts.EpochRef) {
	for id, event := range e.pending {
		if event.ref == ref {
			delete(e.pending, id)
		}
	}
}

func (e *testEvents) advance(to int64) {
	for {
		var chosen contracts.EventID
		at := int64(math.MaxInt64)
		for id, event := range e.pending {
			if event.at <= to && (chosen == 0 || event.at < at || event.at == at && id < chosen) {
				chosen = id
				at = event.at
			}
		}
		if chosen == 0 {
			break
		}
		event := e.pending[chosen]
		delete(e.pending, chosen)
		e.now = max(e.now, event.at)
		event.fn()
	}
	e.now = to
}

type testTrace struct{ names []string }

func (t *testTrace) Emit(_ contracts.EpochRef, name string, _ []contracts.Field) {
	t.names = append(t.names, name)
}
