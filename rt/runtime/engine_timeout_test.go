package gosimruntime

import (
	"errors"
	"math"
	"testing"
)

func TestEngineTimeoutResetUsesVirtualTime(t *testing.T) {
	e, err := NewEngine(EngineConfig{Seed: 1, TimeoutNS: 10, ReplayRevision: ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.SetTimeout(20); !errors.Is(err, ErrEngineState) {
		t.Fatal("inactive timeout", err)
	}
	reached, after := false, false
	err = e.Run(func() {
		if err := e.SetTimeout(30); err != nil {
			t.Fatal(err)
		}
		if err := e.SetTimeout(0); err == nil {
			t.Fatal("zero timeout accepted")
		}
		if err := e.SetTimeout(math.MaxInt64); err == nil {
			t.Fatal("overflow timeout accepted")
		}
		Sleep(20) // The original ten-nanosecond deadline must have been removed.
		reached = true
		if err := e.SetTimeout(5); err != nil {
			t.Fatal(err)
		}
		Sleep(6)
		after = true
	})
	if !errors.Is(err, ErrTimeout) || !reached || after {
		t.Fatalf("timeout err=%v reached=%v after=%v", err, reached, after)
	}
}
