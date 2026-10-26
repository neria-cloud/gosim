package kernel

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	fsprovider "github.com/neria-cloud/gosim/internal/fs"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestFlockUnsupportedAbortsAndClearsViews(t *testing.T) {
	for _, mode := range []string{"shared", "blocking", "posix"} {
		t.Run(mode, func(t *testing.T) {
			e := engineFor(t)
			var k *Kernel
			var inv Invocation
			var returned, recovered atomic.Bool
			lock := contracts.Lock{Type: 1, PID: 99}
			before := lock
			err := e.Run(func() {
				var err error
				k, err = New(configFor(contracts.FeatureSet(contracts.RT|contracts.FS)), e, []contracts.Provider{fsprovider.NewProvider(fsprovider.FlockLinux)})
				if err != nil {
					t.Fatal(err)
				}
				_ = k.Run(func() {
					fd := fsCall(t, k, OpenatRequest{DirFD: -100, Path: "lock", Flags: 0x40 | 2, Mode: 0o600}, DecodeOpenatResult).FD
					switch mode {
					case "shared":
						inv = FlockRequest{FD: fd, How: 1 | 4}.Invocation()
					case "blocking":
						fsCall(t, k, FlockRequest{FD: fd, How: 2}, DecodeFlockResult)
						fd = fsCall(t, k, OpenatRequest{DirFD: -100, Path: "lock", Flags: 2}, DecodeOpenatResult).FD
						inv = FlockRequest{FD: fd, How: 2}.Invocation()
					case "posix":
						inv = FcntlLockRequest{FD: fd, Command: 6, Lock: abi.NewValueView(&lock)}.Invocation()
					}
					inv.Context = currentContext(k)
					inv.Context.PC = 0x1234
					defer func() { _ = recover(); recovered.Store(true) }()
					_ = k.dispatcher.Submit(&inv) // ignoring errors cannot hide unsupported semantics
					returned.Store(true)
				})
			})
			var abort *simrt.AbortError
			if !errors.As(err, &abort) || abort.Diagnostic.Kind != "UnsupportedOperation" || abort.Diagnostic.Feature != "fs" || abort.Diagnostic.PC != 0x1234 {
				t.Fatal(err)
			}
			if returned.Load() || recovered.Load() || !k.closed || lock != before {
				t.Fatal("unsupported lock resumed or mutated payload")
			}
			if inv.Args != ([5]any{}) || inv.Result != (Result{}) || len(k.dispatcher.live) != 0 {
				t.Fatal("borrowed lock view retained")
			}
		})
	}
}
