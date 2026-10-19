// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

//go:build !sim

package race_test

import (
	"runtime"
	"sync/atomic"
	"testing"
)

// This upstream host-GC regression remains native-only. Simulated runtime.GC
// is explicitly rejected by the unsupported script fixture.
func TestNoRaceDeferAtomicStore(t *testing.T) {
	// XXX: this test causes flakes in eg TestRaceChanSendLen
	t.Skip()

	// Test that when an atomic function is deferred directly, the
	// GC scans it correctly. See issue 42599.
	type foo struct {
		bar int64
	}

	var doFork func(f *foo, depth int)
	doFork = func(f *foo, depth int) {
		atomic.StoreInt64(&f.bar, 1)
		defer atomic.StoreInt64(&f.bar, 0)
		if depth > 0 {
			for i := 0; i < 2; i++ {
				f2 := &foo{}
				go doFork(f2, depth-1)
			}
		}
		runtime.GC()
	}

	f := &foo{}
	doFork(f, 11)
}
