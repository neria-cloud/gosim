//go:build go1.25 && !go1.26

package kernel

import (
	"errors"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func TestUnsupportedPrivateProcessHooks(t *testing.T) {
	for name, operation := range map[string]func(){
		"syscall.runtime_BeforeFork":          Syscall_runtime_BeforeFork,
		"syscall.runtime_AfterFork":           Syscall_runtime_AfterFork,
		"syscall.runtime_AfterForkInChild":    Syscall_runtime_AfterForkInChild,
		"syscall.runtime_BeforeExec":          Syscall_runtime_BeforeExec,
		"syscall.runtime_AfterExec":           Syscall_runtime_AfterExec,
		"syscall.hasWaitingReaders":           func() { Syscall_hasWaitingReaders(nil) },
		"syscall.Exit":                        func() { Syscall_Exit(0) },
		"syscall.runtime_doAllThreadsSyscall": func() { Syscall_runtime_doAllThreadsSyscall(0, 0, 0, 0, 0, 0, 0) },
		"syscall.cgocaller":                   func() { Syscall_cgocaller(nil) },
		"syscall.runtime_entersyscall":        Syscall_runtime_entersyscall,
		"syscall.runtime_exitsyscall":         Syscall_runtime_exitsyscall,
		"syscall.gettimeofday":                func() { Syscall_gettimeofday(nil) },
		"golang.org/x/sys/unix.gettimeofday":  func() { GolangOrgXSysUnix_gettimeofday(nil) },
	} {
		t.Run(name, func(t *testing.T) {
			after, deferred := false, false
			err := runAdapters(t, contracts.FeatureSet(contracts.RT), func() { defer func() { _ = recover(); deferred = true }(); operation(); after = true })
			var aborted *simrt.AbortError
			if !errors.As(err, &aborted) || aborted.Diagnostic.Operation != name || after || deferred {
				t.Fatalf("err=%v after=%v deferred=%v", err, after, deferred)
			}
		})
	}
}
