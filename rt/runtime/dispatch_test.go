package gosimruntime

import (
	"bytes"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestNativeDiagnosticSinkAndCanceledSemaphore(t *testing.T) {
	e := testEngine(t)
	var logs bytes.Buffer
	e.scheduler.logger = makeLogger(&logs, slog.LevelInfo)
	var woke atomic.Bool
	var semaphore uint32
	if err := e.Run(func() {
		token, err := e.PrepareMachine(1, 1, "diagnostic owner")
		if err != nil {
			t.Fatal(err)
		}
		ready := NewChan[bool](0)
		e.PublishMachine(token, func() { ready.Send(true); Semacquire(&semaphore, false); woke.Store(true) })
		ready.Recv()
		for range 4 {
			Yield()
		}
		_ = e.onSupervisor(func() {
			if getsema(&semaphore, false) == nil {
				t.Error("fixture did not block in semaphore")
			}
		})
		e.WriteDiagnostic(token, 7, 2, abi.ByteView{Ptr: []byte("hello supervisor\n")})
		e.CancelMachine(token)
		_ = e.onSupervisor(func() {
			for _, head := range semas {
				for cur := head; cur != nil; cur = cur.next {
					if cur.ptr == &semaphore {
						t.Error("native wait retained retired invocation memory")
					}
				}
			}
		})
		e.RetireSemaphore(abi.NewValueView(&semaphore))
		e.DisposeMachine(token)
	}); err != nil {
		t.Fatal(err)
	}
	if woke.Load() {
		t.Fatal("cancellation woke the dead caller")
	}
	if !strings.Contains(logs.String(), "hello supervisor") || !strings.Contains(logs.String(), "stderr") ||
		!strings.Contains(logs.String(), "diagnostic owner") {
		t.Fatal("diagnostic was not emitted through native log sink", logs.String())
	}
}
