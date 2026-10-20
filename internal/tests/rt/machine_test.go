//go:build sim

package behavior_test

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	"github.com/neria-cloud/gosim/rt/runtime"
)

func TestMachineDestroyedHandle(t *testing.T) {
	for _, terminal := range []string{"new", "stopped", "crashed"} {
		t.Run(terminal, func(t *testing.T) {
			m, err := gosim.NewMachine(gosim.MachineConfig{Main: func() { select {} }})
			testsupport.Check(t, err)
			if terminal != "new" {
				testsupport.Check(t, m.Start())
				if terminal == "stopped" {
					testsupport.Check(t, m.Stop())
				} else {
					testsupport.Check(t, m.Crash())
				}
			}
			testsupport.Check(t, m.Destroy())
			if err := m.Destroy(); !errors.Is(err, gosim.ErrMachineNotFound) {
				t.Fatalf("second Destroy: %v", err)
			}
			if _, err := m.State(); !errors.Is(err, gosim.ErrMachineNotFound) {
				t.Fatalf("State after Destroy: %v", err)
			}
		})
	}
}

func TestMachineCrash(t *testing.T) {
	output := make(chan int, 3)
	m := testsupport.Start(t, func() {
		// sleep a second in both the machine and with the crash call to make
		// sure they start "at the same time". all global initialization code
		// runs before.
		time.Sleep(time.Second)
		for i := 1; i <= 3; i++ {
			gosimruntime.Yield()
			output <- i
		}
	})
	time.Sleep(time.Second)
	testsupport.Check(t, m.Crash())
	// Capture progress before the crash without making its distribution depend
	// on the scheduling points inside slog. Report the completed prefix only
	// after the machine can no longer produce output.
	for {
		select {
		case n := <-output:
			slog.Info("output", "n", n)
		default:
			return
		}
	}
}

func TestMachineGo(t *testing.T) {
	global := gosim.CurrentMachine()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if global != gosim.CurrentMachine() {
			t.Error("expected current machine to stay same")
		}
	}()

	wg.Add(1)
	go func() {
		go func() {
			go func() {
				go func() {
					defer wg.Done()
					if global != gosim.CurrentMachine() {
						t.Error("expected current machine to stay same")
					}
				}()
			}()
		}()
	}()

	wg.Add(1)
	testsupport.Start(t, func() {
		inner := gosim.CurrentMachine()
		if inner == global {
			t.Error("expected global to be different from inner")
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			defer wg.Done()
			if inner != gosim.CurrentMachine() {
				t.Error("expected current machine to stay same")
			}
		}()

		// wait until done; if the machine main returns, all goroutines will stop
		// XXX: test that also somehow???
		<-done
	})

	wg.Wait()
}

func TestMachineCurrent(t *testing.T) {
	global := gosim.CurrentMachine()

	if global != gosim.CurrentMachine() {
		t.Error("expected current machine to stay same")
	}

	var inner gosim.Machine
	gosimruntime.TestRaceToken.Release()
	m := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		inner = gosim.CurrentMachine()
		// if inner == nil {
		// t.Error("expected non-nil inner")
		// }
		if inner != gosim.CurrentMachine() {
			t.Error("expected current machine to stay same")
		}
		if inner == global {
			t.Error("expected different global and inner")
		}
		gosimruntime.TestRaceToken.Release()
	})
	testsupport.Wait(t, m)
	gosimruntime.TestRaceToken.Acquire()
	if inner != m {
		t.Error("expected inner to match m")
	}

	var inner2 gosim.Machine
	gosimruntime.TestRaceToken.Release()
	m2 := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		inner2 = gosim.CurrentMachine()
		if inner2 != gosim.CurrentMachine() {
			t.Error("expected current machine to stay same")
		}
		// if inner2 == nil {
		// t.Error("expected non-nil inner2")
		// }
		if inner2 == global {
			t.Error("expected different global and inner2")
		}
		gosimruntime.TestRaceToken.Release()
	})
	testsupport.Wait(t, m2)
	gosimruntime.TestRaceToken.Acquire()
	if inner2 != m2 {
		t.Error("expected inner2 to match m2")
	}

	var inner3 gosim.Machine
	testsupport.Check(t, m2.SetMain(func() {
		gosimruntime.TestRaceToken.Acquire()
		inner3 = gosim.CurrentMachine()
		gosimruntime.TestRaceToken.Release()
	}))
	gosimruntime.TestRaceToken.Release()
	testsupport.Check(t, m2.Restart())
	testsupport.Wait(t, m2)
	gosimruntime.TestRaceToken.Acquire()
	if inner3 != m2 {
		t.Error("expected current machine to stay same")
	}
}

func TestMachineTimer(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	testsupport.Start(t, func() {
		inner := gosim.CurrentMachine()
		go func() {
			time.AfterFunc(time.Second, func() {
				if gosim.CurrentMachine() != inner {
					t.Error("expected timer machine to match inner")
				}
				wg.Done()
			})
		}()
		select {}
	})

	global := gosim.CurrentMachine()
	wg.Add(1)
	go func() {
		time.AfterFunc(time.Second, func() {
			if gosim.CurrentMachine() != global {
				t.Error("expected timer machine to match global")
			}
			wg.Done()
		})
		select {}
	}()

	wg.Wait()
}

var testGlobal = 0

func init() {
	testGlobal = 3
}

func TestMachineGlobals(t *testing.T) {
	if testGlobal != 3 {
		t.Error("expected 3")
	}
	testGlobal = 1

	testsupport.Wait(t, testsupport.Start(t, func() {
		if testGlobal != 3 {
			t.Error("expected 3")
		}
		testGlobal = 2
	}))

	if testGlobal != 1 {
		t.Error("expected 1")
	}
}

func TestMachineLabel(t *testing.T) {
	m := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "hello",
		Main:  func() {},
	})
	if got := testsupport.Label(t, m); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}
