//go:build go1.25

package kernel

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/hostabi"
	kernel "github.com/neria-cloud/gosim/internal/kernel"
	netprovider "github.com/neria-cloud/gosim/internal/net"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func runAdapters(t *testing.T, features contracts.FeatureSet, main func()) error {
	t.Helper()
	e, err := simrt.NewEngine(simrt.EngineConfig{Seed: 17, TimeoutNS: 1e9, ReplayRevision: simrt.ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	return e.Run(func() {
		var providers []contracts.Provider
		if features.Has(contracts.Net) {
			providers = append(providers, netprovider.NewProvider())
		}
		k, err := kernel.New(kernel.Config{Identity: hostabi.Identity(), BuildDigest: strings.Repeat("b", 64), Features: features, TimeoutNS: 1e9}, e, providers)
		if err != nil {
			t.Fatal(err)
		}
		control.Bind(k)
		defer func() {
			if err := k.Close(); err != nil {
				t.Error(err)
			}
			control.Bind(nil)
		}()
		if err := k.Run(func() {
			// Ordinary adapter tests share this global; translated builds isolate it.
			// Fatal fixtures skip defers, so publish the reset before entering main.
			simrt.RaceAcquire(unsafe.Pointer(&process))
			if err := ConfigureProcess(nil, nil); err != nil {
				t.Fatal(err)
			}
			simrt.RaceRelease(unsafe.Pointer(&process))
			main()
			simrt.RaceRelease(unsafe.Pointer(&process))
		}); err != nil {
			t.Error(err)
		}
	})
}

func TestProcessInitializationAndDebug(t *testing.T) {
	t.Setenv("GOSIM_PROCESS_HOOK_TEST", "host-value")
	if err := runAdapters(t, contracts.FeatureSet(contracts.RT), func() {
		if !reflect.DeepEqual(Os_runtime_args(), []string{"gosimapp"}) || len(Syscall_runtime_envs()) != 0 {
			t.Fatal("inherited host process")
		}
		args, env := []string{"app", "arg"}, []string{"GODEBUG=first=1", "LOCAL=value"}
		if err := ConfigureProcess(args, env); err != nil {
			t.Fatal(err)
		}
		args[0] = "mutated"
		env[0] = "mutated"
		copied := Os_runtime_args()
		copied[0] = "mutated"
		copyEnv := Syscall_runtime_envs()
		copyEnv[0] = "mutated"
		if Os_runtime_args()[0] != "app" || Syscall_runtime_envs()[0] != "GODEBUG=first=1" {
			t.Fatal("process state aliases caller")
		}
		var updates []string
		InternalGodebug_setUpdate(func(defaults, env string) {
			if defaults != "" {
				t.Fatal("host debug defaults")
			}
			updates = append(updates, env)
		})
		Syscall_runtimeSetenv("GODEBUG", "second=2")
		Syscall_runtimeUnsetenv("GODEBUG")
		if !reflect.DeepEqual(updates, []string{"first=1", "second=2", ""}) {
			t.Fatal(updates)
		}
		Syscall_runtimeSetenv("GOSIM_PROCESS_HOOK_TEST", "sim-value")
		if os.Getenv("GOSIM_PROCESS_HOOK_TEST") != "host-value" {
			t.Fatal("host env modified")
		}
		called := false
		InternalGodebug_registerMetric("metric", func() uint64 { called = true; return 0 })
		InternalGodebug_setNewIncNonDefault(func(string) func() { called = true; return func() { called = true } })
		if called {
			t.Fatal("metric stub called callback")
		}
		if Syscall_Getpagesize() != 4096 {
			t.Fatal("page size")
		}
		if err := ConfigureProcess(nil, []string{"bad"}); !errors.Is(err, contracts.ErrInvalidConfig) {
			t.Fatal(err)
		}
		if Os_runtime_args()[0] != "app" {
			t.Fatal("invalid configuration mutated state")
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStandardDescriptorsAndPoll(t *testing.T) {
	if err := runAdapters(t, contracts.FeatureSet(contracts.RT), func() {
		for fd, want := range []int{0, 1, 1} {
			out, err := Call(kernel.FcntlIntRequest{FD: fd, Command: 3}, kernel.DecodeFcntlIntResult)
			if err != nil || out.Value != want {
				t.Fatal(fd, out, err)
			}
		}
		InternalPoll_runtime_pollServerInit()
		if InternalPoll_runtime_isPollServerDescriptor(1) || InternalPoll_runtimeNano() != simrt.Nanotime() {
			t.Fatal("native poll state")
		}
		if handle, code := InternalPoll_runtime_pollOpen(2); handle != 0 || code != int(syscall.EPERM) {
			t.Fatal(handle, code)
		}
		if handle, code := InternalPoll_runtime_pollOpen(99); handle != 0 || code != int(syscall.EBADF) {
			t.Fatal(handle, code)
		}
		message := []byte("adapter diagnostic\n")
		if got := InternalGodebug_write(2, unsafe.Pointer(&message[0]), int32(len(message))); got != int32(len(message)) {
			t.Fatal(got)
		}
		if InternalGodebug_write(1, nil, 0) != -1 || InternalGodebug_write(2, nil, 1) != -1 || InternalGodebug_write(2, nil, -1) != -1 {
			t.Fatal("invalid diagnostic write accepted")
		}
		if _, err := Call(kernel.CloseRequest{FD: 1}, kernel.DecodeCloseResult); err != nil {
			t.Fatal(err)
		}
		if _, err := Call(kernel.FcntlIntRequest{FD: 1, Command: 3}, kernel.DecodeFcntlIntResult); err != syscall.EBADF {
			t.Fatal("closed standard descriptor", err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTCPProbeAndPollHandles(t *testing.T) {
	if err := runAdapters(t, contracts.FeatureSet(contracts.RT|contracts.Net), func() {
		out, err := Call(kernel.SocketRequest{Domain: 2, Type: 1 | 0x800 | 0x80000, Protocol: 6}, kernel.DecodeSocketResult)
		if err != nil {
			t.Fatal("IPv4 probe", err)
		}
		if _, err := Call(kernel.SocketRequest{Domain: 10, Type: 1 | 0x800 | 0x80000, Protocol: 6}, kernel.DecodeSocketResult); err != syscall.EAFNOSUPPORT {
			t.Fatal("IPv6 probe", err)
		}
		handle, code := InternalPoll_runtime_pollOpen(uintptr(out.FD))
		if code != 0 || handle == 0 {
			t.Fatal(handle, code)
		}
		InternalPoll_runtime_pollSetDeadline(handle, -1, 'r')
		if code := InternalPoll_runtime_pollWait(handle, 'r'); code != 2 {
			t.Fatal("expired", code)
		}
		InternalPoll_runtime_pollSetDeadline(handle, 0, 'r')
		if code := InternalPoll_runtime_pollReset(handle, 'r'); code != 0 {
			t.Fatal("reset", code)
		}
		InternalPoll_runtime_pollUnblock(handle)
		if code := InternalPoll_runtime_pollWait(handle, 'r'); code != 1 {
			t.Fatal("unblocked", code)
		}
		InternalPoll_runtime_pollClose(handle)
		if _, err := Call(kernel.CloseRequest{FD: out.FD}, kernel.DecodeCloseResult); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFatalAdapterBoundaries(t *testing.T) {
	for name, operation := range map[string]func(){
		"fs":              func() { Require(contracts.FS, "before-pointer-read") },
		"net":             func() { Require(contracts.Net, "before-pointer-read") },
		"invalid-feature": func() { Require(contracts.Feature(255), "invalid") },
		"pidfd":           func() { _ = Os_checkClonePidfd() },
		"exit":            func() { Os_runtime_beforeExit(0) },
		"poll-mode":       func() { InternalPoll_runtime_pollWait(1, 256+'r') },
		"poll-canceled":   func() { InternalPoll_runtime_pollWaitCanceled(1, 'r') },
		"standard-fcntl":  func() { _, _ = Call(kernel.FcntlIntRequest{FD: 1, Command: 4}, kernel.DecodeFcntlIntResult) },
	} {
		t.Run(name, func(t *testing.T) {
			reached, deferred := false, false
			err := runAdapters(t, contracts.FeatureSet(contracts.RT), func() { defer func() { _ = recover(); deferred = true }(); operation(); reached = true })
			if err == nil || reached || deferred {
				t.Fatal("fatal boundary returned/recovered", err, reached, deferred)
			}
		})
	}
}
