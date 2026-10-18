package kernel

import (
	"fmt"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func (k *Kernel) Network(ctx contracts.CallContext) (contracts.SocketOps, error) {
	k.Require(ctx, contracts.Net, "network syscall")
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return nil, err
	}
	if run.bindings.Sockets == nil {
		return nil, contracts.ErrInvalidState
	}
	return run.bindings.Sockets, nil
}

// TODO(migration): Task 14 generates these typed route decoders and manifest.
//
//go:norace
func (d *Dispatcher) decodeSockets(inv *Invocation) (Result, bool, error) {
	ctx, a := inv.Context, inv.Scalars
	var addr contracts.Address
	var view abi.ByteView
	valid := true
	used := 0
	switch inv.Route {
	case RouteBind, RouteConnect:
		used = 1
		addr, valid = inv.Args[0].(contracts.Address)
	case RouteSetOption, RouteGetOption:
		used = 1
		view, valid = inv.Args[0].(abi.ByteView)
	}
	for _, arg := range inv.Args[used:] {
		if arg != nil {
			valid = false
		}
	}
	if !valid {
		return Result{}, false, &contracts.Failure{Code: contracts.InvalidABI, Diagnostic: contracts.Diagnostic{Feature: "net", Operation: "decode", Epoch: ctx.Epoch, Message: "invalid typed network payload"}}
	}
	sockets, err := d.kernel.Network(ctx)
	if err != nil {
		return Result{}, false, err
	}
	var result Result
	var n int
	switch inv.Route {
	case RouteSocket:
		n, err = sockets.Socket(ctx, int(a[0]), int(a[1]), int(a[2]))
		result.R0 = uintptr(n)
	case RouteBind:
		err = sockets.Bind(ctx, int(a[0]), addr)
	case RouteListen:
		err = sockets.Listen(ctx, int(a[0]), int(a[1]))
	case RouteAccept:
		n, addr, err = sockets.Accept(ctx, int(a[0]), int(a[1]))
		result.R0 = uintptr(n)
		result.Value = addr
	case RouteConnect:
		err = sockets.Connect(ctx, int(a[0]), addr)
	case RouteLocalAddress:
		result.Value, err = sockets.LocalAddress(ctx, int(a[0]))
	case RoutePeerAddress:
		result.Value, err = sockets.PeerAddress(ctx, int(a[0]))
	case RouteSetOption:
		err = sockets.SetOption(ctx, int(a[0]), int(a[1]), int(a[2]), view)
	case RouteGetOption:
		n, err = sockets.GetOption(ctx, int(a[0]), int(a[1]), int(a[2]), view)
		result.R0 = uintptr(n)
	default:
		return Result{}, false, fmt.Errorf("unknown network route %d", inv.Route)
	}
	if err != nil {
		errno, ok := err.(syscall.Errno)
		if !ok {
			return Result{}, false, err
		}
		result.Errno = uintptr(errno)
	}
	return result, false, nil
}
