package kernel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestSharedDescriptorCapabilitiesAndFcntlOwnership(t *testing.T) {
	var log []string
	_, _, providers := providersFor(&log)
	_, err := fixture(t, contracts.FeatureSet(contracts.RT|contracts.FS|contracts.Net), providers, func(k *Kernel) {
		ref := createFor(t, k, "io", idleMain)
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		run := k.machineByID[ref.ID].current
		ctx := contracts.CallContext{Epoch: run.ref}
		run.bindings = contracts.Bindings{Files: &fcntlFiles{}, Sockets: &fcntlSockets{}}
		file, socket := &dataResource{owner: contracts.FS}, &dataResource{owner: contracts.Net}
		fileFD, err := run.descriptors.Allocate(file)
		if err != nil {
			t.Fatal(err)
		}
		socketFD, err := run.descriptors.Allocate(socket)
		if err != nil {
			t.Fatal(err)
		}
		if socketFD != fileFD+1 {
			t.Fatal("providers did not share descriptor namespace")
		}
		for _, fd := range []int{fileFD, socketFD} {
			input := []byte("payload")
			if n, err := k.Write(ctx, fd, abi.ByteView{Ptr: input}); n != len(input) || err != nil {
				t.Fatal(n, err)
			}
			input[0] = 'x'
			output := make([]byte, 7)
			if n, err := k.Read(ctx, fd, abi.ByteView{Ptr: output}); n != 7 || err != nil ||
				string(output) != "payload" {
				t.Fatal(n, err, string(output))
			}
		}
		if n, err := k.FcntlInt(ctx, fileFD, 2, 3); n != 1005+fileFD || err != nil {
			t.Fatal("wrong fs fcntl owner", n, err)
		}
		if n, err := k.FcntlInt(ctx, socketFD, 2, 3); n != 2005+socketFD || err != nil {
			t.Fatal("wrong net fcntl owner", n, err)
		}
		file.closeErr = syscall.EIO
		if err := k.CloseFD(ctx, fileFD); !errors.Is(err, syscall.EIO) {
			t.Fatal(err)
		}
		if _, err := k.Read(ctx, fileFD, abi.ByteView{}); !errors.Is(err, syscall.EBADF) {
			t.Fatal("closed descriptor remained live", err)
		}
		if err := k.CloseFD(ctx, fileFD); !errors.Is(err, syscall.EBADF) {
			t.Fatal(err)
		}
		newFD, err := run.descriptors.Allocate(&dataResource{owner: contracts.FS})
		if err != nil || newFD <= socketFD {
			t.Fatal("descriptor reused", newFD, err)
		}
		for _, fd := range []int{-1, math.MaxInt32} {
			if _, err := k.Read(ctx, fd, abi.ByteView{}); !errors.Is(err, syscall.EBADF) {
				t.Fatal(err)
			}
			if _, err := k.Write(ctx, fd, abi.ByteView{}); !errors.Is(err, syscall.EBADF) {
				t.Fatal(err)
			}
		}
		if _, err := k.Read(ctx, 3, abi.ByteView{}); !errors.Is(err, syscall.EBADF) {
			t.Fatal("resource without readable capability", err)
		}
		if err := k.CloseFD(ctx, socketFD); err != nil || socket.closeReason != contracts.CloseExplicit {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDescriptorFinalizeContinuesInOrder(t *testing.T) {
	t.Parallel()
	table := NewFDTable(contracts.EpochRef{})
	var order []int
	var resources []*dataResource
	for range 3 {
		r := &dataResource{owner: contracts.FS, releaseOrder: &order}
		fd, err := table.Allocate(r)
		if err != nil {
			t.Fatal(err)
		}
		r.fd = fd
		resources = append(resources, r)
	}
	resources[0].closeErr, resources[1].closeErr = errors.New("first release"), errors.New("second release")
	err := table.Finalize(contracts.CloseCrash)
	if err == nil || !strings.Contains(err.Error(), "first release") ||
		!strings.Contains(err.Error(), "second release") {
		t.Fatal(err)
	}
	if !slices.Equal(order, []int{3, 4, 5}) || len(table.entries) != 0 {
		t.Fatal("finalization did not drain in order", order)
	}
	if err := table.Finalize(contracts.CloseGraceful); err != nil {
		t.Fatal(err)
	}
	for _, r := range resources {
		if r.released != 1 || r.closeReason != contracts.CloseCrash {
			t.Fatal("duplicate or wrong release")
		}
	}
	if _, err := table.Allocate(&dataResource{owner: contracts.Net}); err == nil {
		t.Fatal("closed table accepted resources")
	}
	table = NewFDTable(contracts.EpochRef{})
	table.next = math.MaxInt32
	if fd, err := table.Allocate(&dataResource{owner: contracts.FS}); err != nil || fd != math.MaxInt32 {
		t.Fatal(fd, err)
	}
	if _, err := table.Allocate(&dataResource{owner: contracts.FS}); !errors.Is(err, syscall.EMFILE) {
		t.Fatal("descriptor overflow", err)
	}
}

func TestRuntimeOnlyDiagnosticsAndProcessServices(t *testing.T) {
	_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ref := createFor(t, k, strings.Repeat("n", 100), idleMain)
		if err := k.Start(ref); err != nil {
			t.Fatal(err)
		}
		ctx := contracts.CallContext{Epoch: k.machineByID[ref.ID].current.ref}
		input := []byte{7}
		if n, err := k.Read(ctx, 0, abi.ByteView{Ptr: input}); n != 0 || err != nil || input[0] != 7 {
			t.Fatal("stdin is not deterministic EOF")
		}
		for _, fd := range []int{1, 2} {
			message := []byte("diagnostic\n")
			if n, err := k.Write(ctx, fd, abi.ByteView{Ptr: message}); n != len(message) || err != nil {
				t.Fatal(n, err)
			}
			message[0] = 'x'
		}
		if len(k.traces) != 2 || k.traces[0].event != "stdout" || k.traces[1].event != "stderr" ||
			k.traces[0].fields[0].Value != "diagnostic\n" {
			t.Fatal("diagnostics were not copied into logs")
		}
		if n, err := k.Write(ctx, 0, abi.ByteView{}); n != 0 || !errors.Is(err, syscall.EBADF) {
			t.Fatal(n, err)
		}
		if err := k.CloseFD(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := k.Write(ctx, 1, abi.ByteView{}); !errors.Is(err, syscall.EBADF) {
			t.Fatal(err)
		}
		if k.Getpid(ctx) != 42 {
			t.Fatal("changed historical pid")
		}
		info, err := k.Uname(ctx)
		if err != nil || info != (ProcessInfo{Nodename: strings.Repeat("n", 64)}) {
			t.Fatal("uname invented unmodeled fields", info, err)
		}
		wantRandom := simrt.NewDomainRandom(42, "application", ref.ID)
		var want [16]byte
		binary.LittleEndian.PutUint64(want[:8], wantRandom.Uint64())
		binary.LittleEndian.PutUint64(want[8:], wantRandom.Uint64())
		got := make([]byte, 10)
		if n, err := k.Getrandom(ctx, abi.ByteView{Ptr: got}, 0); n != 10 || err != nil ||
			!bytes.Equal(got, want[:10]) {
			t.Fatal("getrandom stream mismatch", got, err)
		}
		if _, err := k.Getrandom(ctx, abi.ByteView{Ptr: got}, 1); !errors.Is(err, syscall.EINVAL) {
			t.Fatal("unsupported flags", err)
		}
		capabilities := ProcessCapabilities()
		capabilities[0].Status = "mutated"
		if ProcessCapabilities()[0].Status == "mutated" || ProcessCapabilities()[1].Status != "stub" {
			t.Fatal("capabilities hide limits or share memory")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
