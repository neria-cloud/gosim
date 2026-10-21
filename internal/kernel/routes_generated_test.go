package kernel

import (
	"errors"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestGeneratedPayloadBoundaries(t *testing.T) {
	t.Parallel()
	buffer := []byte("input")
	request := WriteRequest{FD: 1, From: abi.NewSliceView(&buffer[0], uintptr(len(buffer)))}
	inv := request.Invocation()
	request.FD = 2
	if inv.Args[0].(WriteRequest).FD != 1 {
		t.Fatal("request value was not copied")
	}
	if err := validateRoutePayload(&inv); err != nil {
		t.Fatal(err)
	}
	for _, valid := range []Invocation{
		GetpidRequest{}.Invocation(),
		PollWaitRequest{Handle: 9, Mode: 'r'}.Invocation(),
		OpenatRequest{DirFD: -100, Path: "/app/test", Flags: 0, Mode: 0o644}.Invocation(),
		BindRequest{FD: 3, Address: contracts.Address{Family: 2, Port: 80}}.Invocation(),
		CreateRequest{Label: "child", Main: func() {}}.Invocation(),
		FSNextSnapshotRequest{RunID: 1, Iterator: 1}.Invocation(),
		NetSetLinkPolicyRequest{Link: contracts.NetLink{Direction: 1}, Policy: contracts.NetLinkPolicy{SetLatency: true, MinNS: 2, MaxNS: 3}}.Invocation(),
		InfoRequest{}.Invocation(),
		WritevRequest{FD: 1, From: request.From}.Invocation(),
		SetSyscallCountingRequest{Machine: contracts.MachineRef{RunID: 1, ID: 2}, Enabled: true}.Invocation(),
		ResetSyscallStatsRequest{}.Invocation(),
		SyscallStatsRequest{}.Invocation(),
		CountableSyscallsRequest{}.Invocation(),
	} {
		if err := validateRoutePayload(&valid); err != nil {
			t.Fatal(err)
		}
	}
	invalid := []Invocation{
		{Route: RouteRead, Args: [5]any{WriteRequest{}}},
		{Route: RouteRead, Args: [5]any{&ReadRequest{}}},
		{Route: RouteRead, Args: [5]any{func() { t.Fatal("arbitrary callback executed") }}},
		{Route: RouteRead, Args: [5]any{ReadRequest{}, "extra"}},
		{Route: RouteRead, Args: [5]any{ReadRequest{}}, Scalars: [6]uintptr{1}},
		{Route: RouteRead},
		{Route: RouteWritev, Args: [5]any{WriteRequest{}}},
		{Route: RouteSyscallStats, Args: [5]any{CountableSyscallsRequest{}}},
	}
	for _, inv := range invalid {
		var failure *contracts.Failure
		if err := validateRoutePayload(&inv); !errors.As(err, &failure) || failure.Code != contracts.InvalidABI {
			t.Fatalf("invalid payload: %v", err)
		}
	}
	unknown := Invocation{Route: RouteID(0xffffffff)}
	var failure *contracts.Failure
	if err := validateRoutePayload(&unknown); !errors.As(err, &failure) || failure.Code != contracts.UnsupportedOperation {
		t.Fatalf("unknown route: %v", err)
	}
}

func TestGeneratedSyscallFDExtraction(t *testing.T) {
	for _, request := range []interface{ Invocation() Invocation }{
		ReadRequest{FD: 7},
		WriteRequest{FD: 7},
		WritevRequest{FD: 7},
		CloseRequest{FD: 7},
		FcntlIntRequest{FD: 7},
		FcntlLockRequest{FD: 7},
		Getdents64Request{FD: 7},
		FallocateRequest{FD: 7},
		SeekRequest{FD: 7},
		PwriteRequest{FD: 7},
		PreadRequest{FD: 7},
		SyncRequest{FD: 7},
		FstatRequest{FD: 7},
		TruncateRequest{FD: 7},
		FlockRequest{FD: 7},
		MapRequest{FD: 7},
		BindRequest{FD: 7},
		ListenRequest{FD: 7},
		AcceptRequest{FD: 7},
		ConnectRequest{FD: 7},
		LocalAddressRequest{FD: 7},
		PeerAddressRequest{FD: 7},
		SetOptionRequest{FD: 7},
		GetOptionRequest{FD: 7},
		FstatFSRequest{FD: 7},
		FchmodRequest{FD: 7},
		FchownRequest{FD: 7},
	} {
		inv := request.Invocation()
		if err := validateRoutePayload(&inv); err != nil {
			t.Fatal(err)
		}
		if fd, ok := syscallInputFD(&inv); fd != 7 || !ok || !syscallRouteHasFD(inv.Route) {
			t.Fatalf("%s: fd %d %v", inv.Route, fd, ok)
		}
	}
	for _, request := range []interface{ Invocation() Invocation }{
		GetpidRequest{},
		OpenatRequest{DirFD: 7},
		RenameatRequest{OldDirFD: 7, NewDirFD: 7},
		StatatRequest{DirFD: 7},
		ChmodatRequest{DirFD: 7},
		ChownatRequest{DirFD: 7},
		UnlinkatRequest{DirFD: 7},
		MkdiratRequest{DirFD: 7},
		SocketRequest{},
		UnmapRequest{},
		AdviseRequest{},
		PollOpenRequest{FD: 7},
		SyscallStatsRequest{},
	} {
		inv := request.Invocation()
		if err := validateRoutePayload(&inv); err != nil {
			t.Fatal(err)
		}
		if fd, ok := syscallInputFD(&inv); fd != 0 || ok || syscallRouteHasFD(inv.Route) {
			t.Fatalf("%s classified non-primary FD", inv.Route)
		}
	}
}

func TestGeneratedSyscallMonitorResults(t *testing.T) {
	if got, err := DecodeWritevResult(Result{Value: WritevResult{Count: 4}}); err != nil || got.Count != 4 {
		t.Fatal(got, err)
	}
	if _, err := DecodeWritevResult(Result{Value: WriteResult{Count: 4}}); err == nil {
		t.Fatal("write result accepted for writev")
	}
	if _, err := DecodeWritevResult(Result{Errno: uintptr(syscall.EAGAIN)}); !errors.Is(err, syscall.EAGAIN) {
		t.Fatal(err)
	}
	if _, err := DecodeSetSyscallCountingResult(Result{Value: SetSyscallCountingResult{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeResetSyscallStatsResult(Result{Value: ResetSyscallStatsResult{}}); err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeSyscallStatsResult(Result{Value: SyscallStatsResult{Records: []contracts.SyscallRecord{{Name: "read", Count: 1}}}}); err != nil || got.Records[0].Count != 1 {
		t.Fatal(got, err)
	}
	if got, err := DecodeCountableSyscallsResult(Result{Value: CountableSyscallsResult{Syscalls: []contracts.SyscallInfo{{Name: "read"}}}}); err != nil || got.Syscalls[0].Name != "read" {
		t.Fatal(got, err)
	}
	for _, decode := range []func(Result) error{
		func(r Result) error { _, err := DecodeSetSyscallCountingResult(r); return err },
		func(r Result) error { _, err := DecodeResetSyscallStatsResult(r); return err },
		func(r Result) error { _, err := DecodeSyscallStatsResult(r); return err },
		func(r Result) error { _, err := DecodeCountableSyscallsResult(r); return err },
	} {
		if err := decode(Result{Value: contracts.ErrWrongRun}); !errors.Is(err, contracts.ErrWrongRun) {
			t.Fatal(err)
		}
		if err := decode(Result{Value: ReadResult{}}); err == nil {
			t.Fatal("wrong typed result accepted")
		}
	}
}

func TestGeneratedTypedResults(t *testing.T) {
	t.Parallel()
	got, err := DecodeReadResult(Result{Value: ReadResult{Count: 4}})
	if err != nil || got.Count != 4 {
		t.Fatalf("decode=%+v %v", got, err)
	}
	if _, err := DecodeReadResult(Result{Errno: uintptr(syscall.EBADF)}); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
	if _, err := DecodeStartResult(Result{Value: contracts.ErrWrongRun}); !errors.Is(err, contracts.ErrWrongRun) {
		t.Fatal(err)
	}
	for _, invalid := range []Result{
		{},
		{Value: WriteResult{Count: 4}},
		{Value: &ReadResult{Count: 4}},
		{Value: ReadResult{}, R0: 1},
		{Value: ReadResult{}, Errno: uintptr(syscall.EBADF)},
		{Value: contracts.ErrWrongRun, Errno: uintptr(syscall.EBADF)},
	} {
		var failure *contracts.Failure
		if _, err := DecodeReadResult(invalid); !errors.As(err, &failure) || failure.Code != contracts.InvalidABI {
			t.Fatalf("invalid result: %v", err)
		}
	}
}
