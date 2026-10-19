//go:build go1.26 && !go1.27

package rt

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func runHooks(t *testing.T, f func()) {
	t.Helper()
	e, err := gosimruntime.NewEngine(gosimruntime.EngineConfig{Seed: 42, TimeoutNS: int64(time.Second), ReplayRevision: gosimruntime.ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := e.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := e.Run(f); err != nil {
		t.Fatal(err)
	}
}

func TestSynchronizationHooks(t *testing.T) {
	runHooks(t, func() {
		done := gosimruntime.NewChan[struct{}](0)
		var semaphore uint32
		gosimruntime.Go(func() { InternalSync_runtime_SemacquireMutex(&semaphore, false, 0); done.Send(struct{}{}) })
		Sync_runtime_Semrelease(&semaphore, false, 0)
		done.Recv()
		var list NotifyList
		Sync_runtime_notifyListCheck(unsafe.Sizeof(list))
		token := Sync_runtime_notifyListAdd(&list)
		gosimruntime.Go(func() { Sync_runtime_notifyListWait(&list, token); done.Send(struct{}{}) })
		Sync_runtime_notifyListNotifyOne(&list)
		done.Recv()
		token = Sync_runtime_notifyListAdd(&list)
		// Notifications issued before Wait must also be retained.
		Sync_runtime_notifyListNotifyAll(&list)
		Sync_runtime_notifyListWait(&list, token)
		if InternalSync_runtime_canSpin(0) {
			t.Error("cooperative runtime cannot actively spin")
		}
	})
	defer func() {
		if recover() == nil {
			t.Error("notifyList layout mismatch accepted")
		}
	}()
	Sync_runtime_notifyListCheck(1)
}

func TestTimerHooks(t *testing.T) {
	runHooks(t, func() {
		start := Time_runtimeNano()
		sec, nsec, mono := Time_runtimeNow()
		if mono != start || sec*1e9+int64(nsec) != start || Time_runtimeIsBubbled() {
			t.Error("inconsistent virtual time")
		}
		fired := gosimruntime.NewChan[int64](4)
		callback := func(arg any, seq uintptr, delta int64) { arg.(gosimruntime.Chan[int64]).Send(Time_runtimeNano()) }
		timer := Time_newTimer(start+100, 0, callback, fired, nil)
		if !Time_stopTimer(timer) || Time_stopTimer(timer) {
			t.Error("stop state")
		}
		if Time_resetTimer(timer, start+200, 50) {
			t.Error("reset of stopped timer reported active")
		}
		if at := fired.Recv(); at != start+200 {
			t.Fatalf("first fire=%d", at-start)
		}
		if at := fired.Recv(); at != start+250 {
			t.Fatalf("periodic fire=%d", at-start)
		}
		if !Time_stopTimer(timer) {
			t.Error("periodic timer not active")
		}
		before := Time_runtimeNano()
		Time_Sleep(100)
		if Time_runtimeNano() != before+100 {
			t.Error("Sleep did not use virtual clock")
		}
		if fired.Len() != 0 {
			t.Error("stopped timer fired")
		}
	})
}

func TestMapAndRandomHooks(t *testing.T) {
	sample := func() []uint64 {
		var draws []uint64
		runHooks(t, func() {
			draws = []uint64{MathRand_runtime_rand(), MathRandV2_runtime_rand(), HashMaphash_runtime_rand(), InternalSync_runtime_rand(), uint64(Sync_runtime_randn(17))}
			original := gosimruntime.NewMap[int, string]()
			original.Set(1, "original")
			clone := Maps_clone(original).(gosimruntime.Map[int, string])
			clone.Set(1, "clone")
			if original.Get(1) != "original" || clone.Get(1) != "clone" {
				t.Error("map clone shares mutable entries")
			}
		})
		return draws
	}
	first, second := sample(), sample()
	for i := range first {
		if first[i] != second[i] {
			t.Fatal("random hooks are not replayable")
		}
	}
	if first[4] >= 17 {
		t.Fatal("bounded random result")
	}
}

func TestHooksDoNotImportPrivateNativeFacade(t *testing.T) {
	// Generated hook packages live in another module. Only the public native
	// runtime and ABI are legal native dependencies across that boundary.
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(name, "github.com/neria-cloud/gosim/") && name != "github.com/neria-cloud/gosim/rt/runtime" && name != "github.com/neria-cloud/gosim/rt/runtime/abi" {
				t.Errorf("%s imports native/private implementation %s", path, name)
			}
		}
	}
}
