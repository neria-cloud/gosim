package kernel

import (
	"errors"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestGeneratedPayloadBoundaries(t *testing.T) {
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

func TestGeneratedTypedResults(t *testing.T) {
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
