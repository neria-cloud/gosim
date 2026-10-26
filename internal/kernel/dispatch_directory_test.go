package kernel

import (
	"bytes"
	"errors"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	fsprovider "github.com/neria-cloud/gosim/internal/fs"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestSyscallFailureKeepsClassificationAndErrnoBoundary(t *testing.T) {
	failure := &contracts.Failure{Code: contracts.UnsupportedOperation, Diagnostic: contracts.Diagnostic{Feature: "fs", Operation: "getdents64"}}
	result, pending, err := syscallRouteResult(Getdents64Result{}, failure)
	if err != failure || pending || result != (Result{}) {
		t.Fatal(result, pending, err)
	}
	result, pending, err = syscallRouteResult(Getdents64Result{}, syscall.EBADF)
	if err != nil || pending || result.Errno != uintptr(syscall.EBADF) {
		t.Fatal(result, pending, err)
	}
	plain := errors.New("unmodeled failure")
	_, _, err = syscallRouteResult(Getdents64Result{}, plain)
	if !errors.Is(err, plain) || !strings.Contains(err.Error(), "unmodeled syscall error") {
		t.Fatal(err)
	}
}

func TestDirectoryOverflowAbortsWithCallerAndClearsBorrowedView(t *testing.T) {
	e := engineFor(t)
	var k *Kernel
	var descriptors *FDTable
	var inv Invocation
	var epoch contracts.EpochRef
	var returned, recovered atomic.Bool
	buffer := bytes.Repeat([]byte{0xa5}, 32) // enough for one entry, not both
	before := bytes.Clone(buffer)
	err := e.Run(func() {
		var err error
		k, err = New(configFor(contracts.FeatureSet(contracts.RT|contracts.FS)), e, []contracts.Provider{fsprovider.NewProvider(fsprovider.FlockLinux)})
		if err != nil {
			t.Fatal(err)
		}
		_ = k.Run(func() {
			for _, name := range []string{"a", "b"} {
				fsCall(t, k, OpenatRequest{DirFD: -100, Path: name, Flags: 0x40 | 2, Mode: 0o600}, DecodeOpenatResult)
			}
			fd := fsCall(t, k, OpenatRequest{DirFD: -100, Path: "."}, DecodeOpenatResult).FD
			inv = Getdents64Request{FD: fd, Into: abi.ByteView{Ptr: buffer}}.Invocation()
			inv.Context = currentContext(k)
			inv.Context.PC = 0x1234
			epoch = inv.Context.Epoch
			descriptors = k.machineByID[epoch.Machine.ID].current.descriptors
			defer func() { _ = recover(); recovered.Store(true) }()
			_ = k.dispatcher.Submit(&inv)
			returned.Store(true)
		})
	})
	var abort *simrt.AbortError
	if !errors.As(err, &abort) {
		t.Fatal("overflow was not fatal", err)
	}
	d := abort.Diagnostic
	if d.Kind != "UnsupportedOperation" || d.Feature != "fs" || d.Operation != "getdents64" || d.RunID != epoch.Machine.RunID || d.MachineID != epoch.Machine.ID || d.Epoch != epoch.Epoch || d.PC != 0x1234 {
		t.Fatal(d, epoch)
	}
	if returned.Load() || recovered.Load() || !k.closed {
		t.Fatal("overflow escaped fatal cleanup")
	}
	if inv.Args != ([5]any{}) || inv.Result != (Result{}) || inv.Sema != 0 || len(k.dispatcher.live) != 0 {
		t.Fatal("aborted invocation retained borrowed memory")
	}
	if descriptors == nil || !descriptors.closed || len(descriptors.entries) != 0 {
		t.Fatal("aborted run retained descriptors")
	}
	if !bytes.Equal(buffer, before) {
		t.Fatal("overflow wrote a partial directory")
	}
}
