package kernel

import (
	"encoding/binary"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	netprovider "github.com/neria-cloud/gosim/internal/net"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func netCall(t *testing.T, k *Kernel, route RouteID, scalars [6]uintptr, args [5]any, want syscall.Errno) Result {
	t.Helper()
	inv := &Invocation{Context: currentContext(k), Route: route, Scalars: scalars, Args: args}
	if err := k.dispatcher.Submit(inv); err != nil {
		t.Fatal(err)
	}
	result := inv.TakeResult()
	if result.Errno != uintptr(want) {
		t.Fatalf("route %d errno=%d want=%d", route, result.Errno, want)
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
			create := func() uintptr { return netCall(t, k, RouteSocket, [6]uintptr{2, flags}, [5]any{}, 0).R0 }
			listener := create()
			addr := contracts.Address{Family: 2, IPv4: [4]byte{11, 0, 0, 1}, Port: 80}
			netCall(t, k, RouteBind, [6]uintptr{listener}, [5]any{addr}, 0)
			netCall(t, k, RouteListen, [6]uintptr{listener, 128}, [5]any{}, 0)
			actual := netCall(t, k, RouteLocalAddress, [6]uintptr{listener}, [5]any{}, 0).Value.(contracts.Address)
			if actual != addr {
				t.Fatal(actual)
			}
			listenerPoll := netCall(t, k, RoutePollOpen, [6]uintptr{listener}, [5]any{}, 0).R1
			netCall(t, k, RouteAccept, [6]uintptr{listener, 0x800 | 0x80000}, [5]any{}, syscall.EWOULDBLOCK)
			client := create()
			clientPoll := netCall(t, k, RoutePollOpen, [6]uintptr{client}, [5]any{}, 0).R1
			netCall(t, k, RouteConnect, [6]uintptr{client}, [5]any{addr}, syscall.EINPROGRESS)
			if result := netCall(t, k, RoutePollWait, [6]uintptr{listenerPoll, 'r'}, [5]any{}, 0); result.R0 != 0 {
				t.Fatal(result)
			}
			accepted := netCall(t, k, RouteAccept, [6]uintptr{listener, 0x800 | 0x80000}, [5]any{}, 0)
			if accepted.Value.(contracts.Address).IPv4 != addr.IPv4 {
				t.Fatal(accepted.Value)
			}
			if result := netCall(t, k, RoutePollWait, [6]uintptr{clientPoll, 'w'}, [5]any{}, 0); result.R0 != 0 {
				t.Fatal(result)
			}
			status := make([]byte, 4)
			netCall(t, k, RouteGetOption, [6]uintptr{client, 1, 4}, [5]any{abi.ByteView{Ptr: status}}, 0)
			if binary.LittleEndian.Uint32(status) != 0 {
				t.Fatal("connect incomplete", status)
			}
			netCall(t, k, RouteSetOption, [6]uintptr{client, 6, 1}, [5]any{abi.ByteView{Ptr: status}}, 0)
			netCall(t, k, RouteSetOption, [6]uintptr{client, 6, 999}, [5]any{abi.ByteView{Ptr: status}}, syscall.ENOPROTOOPT)
			netCall(t, k, RouteFcntlInt, [6]uintptr{client, 3}, [5]any{}, syscall.ENOSYS)
			if peer := netCall(t, k, RoutePeerAddress, [6]uintptr{client}, [5]any{}, 0).Value.(contracts.Address); peer != addr {
				t.Fatal(peer)
			}
			serverPoll := netCall(t, k, RoutePollOpen, [6]uintptr{accepted.R0}, [5]any{}, 0).R1
			netCall(t, k, RouteRead, [6]uintptr{accepted.R0}, [5]any{abi.ByteView{Ptr: status}}, syscall.EWOULDBLOCK)
			netCall(t, k, RouteWrite, [6]uintptr{client}, [5]any{abi.ByteView{Ptr: []byte("test")}}, 0)
			if result := netCall(t, k, RoutePollWait, [6]uintptr{serverPoll, 'r'}, [5]any{}, 0); result.R0 != 0 {
				t.Fatal(result)
			}
			if n := netCall(t, k, RouteRead, [6]uintptr{accepted.R0}, [5]any{abi.ByteView{Ptr: status}}, 0).R0; n != 4 || string(status) != "test" {
				t.Fatal(n, string(status))
			}
			netCall(t, k, RoutePollReset, [6]uintptr{serverPoll, 'r'}, [5]any{}, 0)
			netCall(t, k, RoutePollDeadline, [6]uintptr{serverPoll, 100, 'r'}, [5]any{}, 0)
			if result := netCall(t, k, RoutePollWait, [6]uintptr{serverPoll, 'r'}, [5]any{}, 0); result.R0 != 2 {
				t.Fatal("deadline did not wake real caller", result)
			}
			netCall(t, k, RoutePollDeadline, [6]uintptr{serverPoll, 0, 'r'}, [5]any{}, 0)
			netCall(t, k, RouteClose, [6]uintptr{client}, [5]any{}, 0)
			if result := netCall(t, k, RoutePollWait, [6]uintptr{serverPoll, 'r'}, [5]any{}, 0); result.R0 != 0 {
				t.Fatal(result)
			}
			netCall(t, k, RouteRead, [6]uintptr{accepted.R0}, [5]any{abi.ByteView{Ptr: status}}, syscall.EPIPE)
			for _, id := range []uintptr{listenerPoll, clientPoll, serverPoll} {
				netCall(t, k, RoutePollClose, [6]uintptr{id}, [5]any{}, 0)
			}
			netCall(t, k, RouteSocket, [6]uintptr{10, flags}, [5]any{}, syscall.EAFNOSUPPORT)
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
					inv := &Invocation{Context: currentContext(k), Route: RouteSocket, Scalars: [6]uintptr{2, 1 | 0x800 | 0x80000}}
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
