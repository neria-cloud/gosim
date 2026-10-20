package kernel

import (
	"encoding/binary"
	"errors"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	netprovider "github.com/neria-cloud/gosim/internal/net"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func netCall[T any](t *testing.T, k *Kernel, request interface{ Invocation() Invocation }, want syscall.Errno, decode func(Result) (T, error)) T {
	t.Helper()
	inv := request.Invocation()
	inv.Context = currentContext(k)
	if err := k.dispatcher.Submit(&inv); err != nil {
		t.Fatal(err)
	}
	if inv.Args != ([5]any{}) {
		t.Fatal("completion retained request payload")
	}
	result, err := decode(inv.TakeResult())
	if want == 0 && err != nil || want != 0 && !errors.Is(err, want) {
		t.Fatalf("route %d: error %v want %v", inv.Route, err, want)
	}
	if inv.Result != (Result{}) {
		t.Fatal("result retained after extraction")
	}
	return result
}

func TestNetworkApplicationDispatchAndPollDeadline(t *testing.T) {
	e := engineFor(t)
	var completed atomic.Bool
	err := e.Run(func() {
		k, err := New(configFor(contracts.FeatureSet(contracts.RT|contracts.Net)), e, []contracts.Provider{netprovider.NewProvider()})
		if err != nil {
			t.Fatal(err)
		}
		if err := k.controls.NetControl.Configure(contracts.NetConfig{MinNS: 10, MaxNS: 10, Lambda: 1, Repair: 1}); err != nil {
			t.Fatal(err)
		}
		err = k.Run(func() {
			const flags = 1 | 0x800 | 0x80000
			create := func() int {
				return netCall(t, k, SocketRequest{Domain: int(2), Type: int(flags), Protocol: int(0)}, 0, DecodeSocketResult).FD
			}
			listener := create()
			addr := contracts.Address{Family: 2, IPv4: [4]byte{11, 0, 0, 1}, Port: 80}
			netCall(t, k, BindRequest{FD: int(listener), Address: addr}, 0, DecodeBindResult)
			netCall(t, k, ListenRequest{FD: int(listener), Backlog: int(128)}, 0, DecodeListenResult)
			actual := netCall(t, k, LocalAddressRequest{FD: int(listener)}, 0, DecodeLocalAddressResult).Address
			if actual != addr {
				t.Fatal(actual)
			}
			listenerPoll := netCall(t, k, PollOpenRequest{FD: int(listener)}, 0, DecodePollOpenResult).Handle
			netCall(t, k, AcceptRequest{FD: int(listener), Flags: int(0x800 | 0x80000)}, syscall.EWOULDBLOCK, DecodeAcceptResult)
			client := create()
			clientPoll := netCall(t, k, PollOpenRequest{FD: int(client)}, 0, DecodePollOpenResult).Handle
			netCall(t, k, ConnectRequest{FD: int(client), Address: addr}, syscall.EINPROGRESS, DecodeConnectResult)
			if result := netCall(t, k, PollWaitRequest{Handle: uint64(listenerPoll), Mode: uint8('r')}, 0, DecodePollWaitResult); result.Code != 0 {
				t.Fatal(result)
			}
			accepted := netCall(t, k, AcceptRequest{FD: int(listener), Flags: int(0x800 | 0x80000)}, 0, DecodeAcceptResult)
			if accepted.Address.IPv4 != addr.IPv4 {
				t.Fatal(accepted.Address)
			}
			if result := netCall(t, k, PollWaitRequest{Handle: uint64(clientPoll), Mode: uint8('w')}, 0, DecodePollWaitResult); result.Code != 0 {
				t.Fatal(result)
			}
			status := make([]byte, 4)
			netCall(t, k, GetOptionRequest{FD: int(client), Level: int(1), Name: int(4), Into: abi.ByteView{Ptr: status}}, 0, DecodeGetOptionResult)
			if binary.LittleEndian.Uint32(status) != 0 {
				t.Fatal("connect incomplete", status)
			}
			netCall(t, k, SetOptionRequest{FD: int(client), Level: int(6), Name: int(1), Value: abi.ByteView{Ptr: status}}, 0, DecodeSetOptionResult)
			netCall(t, k, SetOptionRequest{FD: int(client), Level: int(6), Name: int(999), Value: abi.ByteView{Ptr: status}}, syscall.ENOPROTOOPT, DecodeSetOptionResult)
			netCall(t, k, FcntlIntRequest{FD: int(client), Command: int(3), Argument: int(0)}, syscall.ENOSYS, DecodeFcntlIntResult)
			if peer := netCall(t, k, PeerAddressRequest{FD: int(client)}, 0, DecodePeerAddressResult).Address; peer != addr {
				t.Fatal(peer)
			}
			serverPoll := netCall(t, k, PollOpenRequest{FD: int(accepted.FD)}, 0, DecodePollOpenResult).Handle
			netCall(t, k, ReadRequest{FD: int(accepted.FD), Into: abi.ByteView{Ptr: status}}, syscall.EWOULDBLOCK, DecodeReadResult)
			netCall(t, k, WriteRequest{FD: int(client), From: abi.ByteView{Ptr: []byte("test")}}, 0, DecodeWriteResult)
			if result := netCall(t, k, PollWaitRequest{Handle: uint64(serverPoll), Mode: uint8('r')}, 0, DecodePollWaitResult); result.Code != 0 {
				t.Fatal(result)
			}
			if n := netCall(t, k, ReadRequest{FD: int(accepted.FD), Into: abi.ByteView{Ptr: status}}, 0, DecodeReadResult).Count; n != 4 || string(status) != "test" {
				t.Fatal(n, string(status))
			}
			netCall(t, k, PollResetRequest{Handle: uint64(serverPoll), Mode: uint8('r')}, 0, DecodePollResetResult)
			netCall(t, k, PollDeadlineRequest{Handle: uint64(serverPoll), DeadlineNS: int64(100), Mode: uint8('r')}, 0, DecodePollDeadlineResult)
			if result := netCall(t, k, PollWaitRequest{Handle: uint64(serverPoll), Mode: uint8('r')}, 0, DecodePollWaitResult); result.Code != 2 {
				t.Fatal("deadline did not wake real caller", result)
			}
			netCall(t, k, PollDeadlineRequest{Handle: uint64(serverPoll), DeadlineNS: int64(0), Mode: uint8('r')}, 0, DecodePollDeadlineResult)
			netCall(t, k, CloseRequest{FD: int(client)}, 0, DecodeCloseResult)
			if result := netCall(t, k, PollWaitRequest{Handle: uint64(serverPoll), Mode: uint8('r')}, 0, DecodePollWaitResult); result.Code != 0 {
				t.Fatal(result)
			}
			netCall(t, k, ReadRequest{FD: int(accepted.FD), Into: abi.ByteView{Ptr: status}}, syscall.EPIPE, DecodeReadResult)
			for _, id := range []uint64{listenerPoll, clientPoll, serverPoll} {
				netCall(t, k, PollCloseRequest{Handle: uint64(id)}, 0, DecodePollCloseResult)
			}
			netCall(t, k, SocketRequest{Domain: int(10), Type: int(flags), Protocol: int(0)}, syscall.EAFNOSUPPORT, DecodeSocketResult)
			completed.Store(true)
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(k.events) != 0 || len(k.polls) != 0 || len(k.dispatcher.live) != 0 {
			t.Fatal("kernel retained network work")
		}
		for _, m := range k.machineByID {
			if m.current != nil {
				t.Fatal("surviving network epoch")
			}
		}
	})
	if err != nil || !completed.Load() {
		t.Fatal("application network dispatch failed", err)
	}
}

func TestNetworkRouteRejectsDisabledAndMalformed(t *testing.T) {
	for _, mode := range []string{"disabled", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			e := engineFor(t)
			err := e.Run(func() {
				features := contracts.FeatureSet(contracts.RT)
				var providers []contracts.Provider
				if mode == "malformed" {
					features |= contracts.FeatureSet(contracts.Net)
					providers = []contracts.Provider{netprovider.NewProvider()}
				}
				k, err := New(configFor(features), e, providers)
				if err != nil {
					t.Fatal(err)
				}
				_ = k.Run(func() {
					inv := &Invocation{Context: currentContext(k), Route: RouteSocket, Args: [5]any{SocketRequest{Domain: 2, Type: 1 | 0x800 | 0x80000}}}
					if mode == "malformed" {
						inv.Route = RouteConnect
						inv.Args[0] = func() {}
					}
					_ = k.dispatcher.Submit(inv)
					t.Error("fatal network call resumed")
				})
			})
			want := "FeatureDisabled"
			if mode == "malformed" {
				want = "InvalidABI"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatal(err)
			}
		})
	}
}
