package contracts

import "github.com/neria-cloud/gosim/rt/runtime/abi"

// ListenerBacklog is the bounded accept queue capacity exposed by net adapters.
const ListenerBacklog = 5

type Address struct {
	Family uint16
	IPv4   [4]byte
	Port   uint16
}
type SocketOps interface {
	Socket(CallContext, int, int, int) (int, error)
	Bind(CallContext, int, Address) error
	Listen(CallContext, int, int) error
	Accept(CallContext, int, int) (int, Address, error)
	Connect(CallContext, int, Address) error
	LocalAddress(CallContext, int) (Address, error)
	PeerAddress(CallContext, int) (Address, error)
	SetOption(CallContext, int, int, int, abi.ByteView) error
	GetOption(CallContext, int, int, int, abi.ByteView) (int, error)
	FcntlInt(CallContext, int, int, int) (int, error)
}
