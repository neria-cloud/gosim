//go:build sim

package behavior_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	"github.com/neria-cloud/gosim/rt/runtime"
)

func TestCrashTimer(t *testing.T) {
	var before, after atomic.Bool

	// token to allow machine to read before, after
	gosimruntime.TestRaceToken.Release()

	m1 := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		// test a timer can fire
		time.AfterFunc(1*time.Second, func() {
			before.Store(true)
		})
		select {}
	})

	m2 := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		// test a crashed timer does not fire
		time.AfterFunc(5*time.Second, func() {
			after.Store(true)
			panic("help")
		})
		select {}
	})

	time.Sleep(2 * time.Second)
	testsupport.Check(t, m1.Crash())
	testsupport.Check(t, m2.Crash())
	time.Sleep(10 * time.Second)

	if !before.Load() {
		t.Error("expected before")
	}
	if after.Load() {
		t.Error("did not expect after")
	}
}

func TestCrashTimersComplicatedHeap(t *testing.T) {
	var timers []*time.Timer
	for i := 0; i < 10; i++ {
		timers = append(timers, time.NewTimer(time.Duration(i+1)*time.Second))
	}

	m := testsupport.Start(t, func() {
		var timers []*time.Timer
		for i := 0; i < 10; i++ {
			timers = append(timers, time.NewTimer(time.Duration(i+1)*time.Second))
		}
		select {}
	})
	time.Sleep(time.Millisecond)
	testsupport.Check(t, m.Crash())
	for i, timer := range timers {
		if i%2 == 0 {
			timer.Stop()
		}
	}
	// XXX: when the heap indexes are not adjusted this test times out???
	// (that is "h.timers[j].pos = j")
	// XXX: check timer heap invariants
}

func TestCrashSleep(t *testing.T) {
	var before, after atomic.Bool

	// token to allow machine to read before, after
	gosimruntime.TestRaceToken.Release()

	m1 := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		// test Sleep works
		time.Sleep(1 * time.Second)
		before.Store(true)
	})

	m2 := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		// test a crashed Sleep stops
		time.Sleep(5 * time.Second)
		after.Store(true)
		panic("help")
	})

	time.Sleep(2 * time.Second)
	testsupport.Check(t, m1.Crash())
	testsupport.Check(t, m2.Crash())
	time.Sleep(10 * time.Second)

	if !before.Load() {
		t.Error("expected before")
	}
	if after.Load() {
		t.Error("did not expect after")
	}
}

func TestCrashSemaphore(t *testing.T) {
	var before, after atomic.Bool
	var sema uint32

	// token to allow machine to read before, after, sema
	gosimruntime.TestRaceToken.Release()

	m1 := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()

		// test that we can acquire the semaphore
		gosimruntime.Semacquire(&sema, false)
		before.Store(true)
	})

	m2 := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()

		// test that an acquire can be crashed
		time.Sleep(2 * time.Second)
		gosimruntime.Semacquire(&sema, false)
		after.Store(true)
	})

	time.Sleep(1 * time.Second)
	// release for m1
	gosimruntime.Semrelease(&sema)

	time.Sleep(3 * time.Second)
	testsupport.Check(t, m1.Crash())
	testsupport.Check(t, m2.Crash())

	// after crash make sure sema still works
	gosimruntime.Semrelease(&sema)
	time.Sleep(time.Second)
	gosimruntime.Semacquire(&sema, false)

	if !before.Load() {
		t.Error("expected before")
	}
	if after.Load() {
		t.Error("did not expect after")
	}
}

func TestCrashChan(t *testing.T) {
	ch1 := make(chan int, 1)
	ch2 := make(chan int, 1)

	m := testsupport.Start(t, func() {
		// make sure we can recv and send channels
		ch2 <- <-ch1 + 1
		// then hang
		<-ch1
	})

	time.Sleep(time.Second)

	// successful recv and send
	ch1 <- 1
	if v := <-ch2; v != 2 {
		t.Error(v)
	}

	// wait a bit to make sure it's in the recv and crash
	time.Sleep(2 * time.Second)
	testsupport.Check(t, m.Crash())
	time.Sleep(2 * time.Second)

	// test channel still works and recv won't consume our value
	ch1 <- 3
	time.Sleep(5 * time.Second)
	if v := <-ch1; v != 3 {
		t.Error(v)
	}
}

func TestCrashSelect(t *testing.T) {
	ch1 := make(chan int, 1)
	ch2 := make(chan int, 0)

	m := testsupport.Start(t, func() {
		// make sure that select works
		select {
		case x := <-ch1:
			ch2 <- x + 1
		case <-ch2:
		}
		// then hang
		select {
		case <-ch1:
		case ch2 <- 1:
		}
	})

	time.Sleep(2 * time.Second)
	ch1 <- 1
	if v := <-ch2; v != 2 {
		t.Error(v)
	}

	// make sure we are in the select and crash
	time.Sleep(time.Second)
	testsupport.Check(t, m.Crash())
	time.Sleep(time.Second)

	// make sure chans still work
	go func() {
		ch2 <- <-ch1 + 1
	}()
	ch1 <- 4
	if v := <-ch2; v != 5 {
		t.Error(v)
	}
}

func TestCrashSyscall(t *testing.T) {
	var before, after, finally atomic.Bool

	// sleeper is a helper machine that will successfully stop
	sleeper := testsupport.Start(t, func() {
		time.Sleep(5 * time.Second)
	})

	// working machine
	gosimruntime.TestRaceToken.Release()
	waiter := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		// wait for sleeper, flag success
		testsupport.Wait(t, sleeper)
		before.Store(true)
		// then sleep, flag success after
		time.Sleep(5 * time.Second)
		finally.Store(true)
	})

	gosimruntime.TestRaceToken.Release()
	crasher := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		// wait for waiter, but will get crashed before it's done
		testsupport.Wait(t, waiter)
		after.Store(true)
	})

	// see waiter work
	time.Sleep(2 * time.Second)
	if before.Load() {
		t.Error("did not yet expect before")
	}
	time.Sleep(4 * time.Second)
	if !before.Load() {
		t.Error("expected before")
	}

	// then crash crasher
	testsupport.Check(t, crasher.Crash())
	testsupport.Wait(t, crasher)
	// make sure after did not trigger
	if after.Load() {
		t.Error("did not expect after")
	}

	// make sure waiter is still sleeping
	if finally.Load() {
		t.Error("did not yet expect finally")
	}
	// make sure wait works
	testsupport.Wait(t, waiter)
	if !finally.Load() {
		t.Error("did expect finally")
	}
}

func TestCrashStressLightly(t *testing.T) {
	// small stress test that creates a bunch of machines interacting with
	// goroutines, timers, and channels, and crashes them hopefully catching
	// reuse problems
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		// create goroutines every 2 seconds
		time.Sleep(2 * time.Second)
		wg.Add(1)
		go func() {
			defer wg.Done()
			var before, after atomic.Bool
			gosimruntime.TestRaceToken.Release()
			// spawn a new machine that we will run for a bit and then crash
			m := testsupport.Start(t, func() {
				gosimruntime.TestRaceToken.Acquire()
				// another goroutine for good measure
				go func() {
					// sleep a bit
					time.Sleep(1 * time.Second)
					ch := make(chan struct{}, 0)
					// make a timer writer to a channel later
					time.AfterFunc(1*time.Second, func() {
						ch <- struct{}{}
					})
					go func() {
						// sleep a bit then try and then have a timer try to write after
						time.Sleep(1 * time.Second)
						time.AfterFunc(3*time.Second, func() {
							after.Store(true)
						})
					}()
					go func() {
						// sleep a bit and then wait for a chan to maybe write after.
						// chan queues are FIFO so the code after will get to recv.
						time.Sleep(1 * time.Second)
						<-ch
						after.Store(true)
					}()
					// recv and then flag success.
					<-ch
					before.Store(true)
				}()
				select {}
			})
			// run for long enough to allow before to succeed.
			time.Sleep(3 * time.Second)
			testsupport.Check(t, m.Crash())
			if !before.Load() {
				t.Error("expected before")
			}
			// allow any lingering code to trigger
			time.Sleep(10 * time.Second)
			if after.Load() {
				t.Error("did not expect after")
			}
		}()
	}
	// ensure all succeed
	wg.Wait()
}

var crashTestGlobal int

func TestCrashRestartGlobals(t *testing.T) {
	var first atomic.Bool
	gosimruntime.TestRaceToken.Release()
	m := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		if crashTestGlobal != 0 {
			t.Error("expect 0")
		}
		crashTestGlobal = 1
		if crashTestGlobal != 1 {
			t.Error("expect 1")
		}
		first.Store(true)
	})
	time.Sleep(time.Second)
	testsupport.Check(t, m.Crash())
	if !first.Load() {
		t.Error("expected first")
	}
	var second atomic.Bool
	gosimruntime.TestRaceToken.Release()
	testsupport.Check(t, m.SetMain(func() {
		gosimruntime.TestRaceToken.Acquire()
		if crashTestGlobal != 0 {
			t.Error("expect 0")
		}
		crashTestGlobal = 1
		if crashTestGlobal != 1 {
			t.Error("expect 1")
		}
		second.Store(true)
	}))
	testsupport.Check(t, m.Restart())
	time.Sleep(time.Second)
	if !second.Load() {
		t.Error("expected second")
	}
}

func TestCrashRestartBootProgram(t *testing.T) {
	var done atomic.Bool
	gosimruntime.TestRaceToken.Release()

	m := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		done.Store(true)
	})

	// give m time to change its boot program
	time.Sleep(time.Second)
	testsupport.Check(t, m.Crash())
	if !done.Load() {
		t.Fatal("expected done")
	}
	done.Store(false)
	testsupport.Check(t, m.Restart())

	time.Sleep(time.Second)
	if !done.Load() {
		t.Fatal("expected done")
	}
}

func TestCrashRestartNewBootProgram(t *testing.T) {
	var done atomic.Bool
	gosimruntime.TestRaceToken.Release()
	m := testsupport.Start(t, func() {
		if err := gosim.CurrentMachine().SetMain(func() { done.Store(true) }); !errors.Is(err, gosim.ErrInvalidState) {
			t.Fatal("running main replacement", err)
		}
	})
	testsupport.Wait(t, m)
	if done.Load() {
		t.Fatal("running machine changed its main")
	}
	testsupport.Check(t, m.SetMain(func() { gosimruntime.TestRaceToken.Acquire(); done.Store(true) }))
	testsupport.Check(t, m.Restart())
	testsupport.Wait(t, m)
	if !done.Load() {
		t.Fatal("stopped machine did not accept new main")
	}
}
