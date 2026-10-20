//go:build sim

package behavior_test

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestAtomicPollingAllowsProgress(t *testing.T) {
	start := make(chan struct{})
	var ready atomic.Bool
	go func() {
		<-start
		ready.Store(true)
	}()
	close(start)
	for range 10000 {
		if ready.Load() {
			return
		}
	}
	t.Fatal("atomic polling starved a runnable goroutine")
}

func TestMutexPollingAllowsProgress(t *testing.T) {
	start := make(chan struct{})
	var mu sync.Mutex
	ready := false
	go func() {
		<-start
		mu.Lock()
		ready = true
		mu.Unlock()
	}()
	close(start)
	for range 10000 {
		mu.Lock()
		done := ready
		mu.Unlock()
		if done {
			return
		}
	}
	t.Fatal("uncontended mutex polling starved a runnable goroutine")
}
