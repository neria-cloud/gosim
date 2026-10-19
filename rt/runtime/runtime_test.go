package gosimruntime

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsSim(t *testing.T) {
	if IsSim() {
		t.Error("should not be sim")
	}
}

func dump() string {
	var buf [32 * 1024]byte
	return string(buf[:runtime.Stack(buf[:], true)])
}

const baseNumGoroutines = 2

func expectNumGoroutines(t *testing.T, situation string) {
	t.Helper()

	// XXX: give any pre-existing goroutines time to fully exit
	start := time.Now()
	for runtime.NumGoroutine() != baseNumGoroutines {
		if time.Since(start) >= 1*time.Second {
			t.Errorf("too many goroutines at %s, expected %d", situation, baseNumGoroutines)
			t.Log(dump())
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// TODO: Rewrite to use another API? Could add a call to dump goroutines to metatesting...?
func TestCleanupGoroutines(t *testing.T) {
	// Count only this test's goroutines: t.Parallel leaves other test goroutines
	// parked in the parent process even while a serial test is running.
	if os.Getenv("GOSIM_CLEANUP_TEST_CHILD") != "1" {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCleanupGoroutines$")
		cmd.Env = append(os.Environ(), "GOSIM_CLEANUP_TEST_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated cleanup check: %v\n%s", err, output)
		}
		return
	}
	expectNumGoroutines(t, "start")

	var hit atomic.Int32
	var deferred atomic.Int32
	f := func() {
		waiting := NewChan[struct{}](0)
		pause := NewChan[struct{}](0)
		for i := 0; i < 5; i++ {
			Go(func() {
				defer func() {
					deferred.Add(1)
				}()

				waiting.Recv() // will not block
				hit.Add(1)
				pause.Recv() // should block
			})
			waiting.Send(struct{}{})
		}
	}
	e := testEngine(t)
	if err := e.Run(func() {
		token, err := e.PrepareMachine(1, 1, "test")
		if err != nil {
			t.Fatal(err)
		}
		done := NewChan[struct{}](0)
		e.PublishMachine(token, func() { f(); done.Send(struct{}{}) })
		done.Recv()
		e.CancelMachine(token)
		e.DisposeMachine(token)
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	expectNumGoroutines(t, "end")
	if hit.Load() != 5 {
		t.Error("recv not hit")
	}
	if deferred.Load() != 0 {
		t.Error("deferred hit")
	}
}
