package contracts

import "github.com/neria-cloud/gosim/rt/runtime/abi"

type Resource interface {
	Owner() Feature
	Release(CloseReason) error
}
type Readable interface {
	Read(CallContext, abi.ByteView) (int, error)
}
type Writable interface {
	Write(CallContext, abi.ByteView) (int, error)
}
type Pollable interface {
	Subscribe(CallContext, ReadinessSink) error
	Unsubscribe(CallContext, ReadinessSink)
}
type DescriptorTable interface {
	Allocate(Resource) (int, error)
	Lookup(int) (Resource, error)
	Remove(int) (Resource, error)
}
