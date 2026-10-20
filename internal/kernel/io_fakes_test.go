package kernel

import (
	"errors"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type dataResource struct {
	owner        contracts.Feature
	data         []byte
	fd           int
	epoch        contracts.EpochRef
	sinks        map[contracts.ReadinessSink]bool
	released     int
	closeReason  contracts.CloseReason
	closeErr     error
	readErr      error
	subscribeErr error
	releaseOrder *[]int
	ready        contracts.Readiness
	onSubscribe  func(contracts.CallContext, contracts.ReadinessSink)
}

func (r *dataResource) Owner() contracts.Feature { return r.owner }
func (r *dataResource) Release(reason contracts.CloseReason) error {
	r.released++
	r.closeReason = reason
	if r.releaseOrder != nil {
		*r.releaseOrder = append(*r.releaseOrder, r.fd)
	}
	if r.released != 1 {
		return errors.New("resource released more than once")
	}
	return r.closeErr
}

func (r *dataResource) Read(_ contracts.CallContext, into abi.ByteView) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	if r.released != 0 {
		return 0, syscall.EBADF
	}
	n := into.Write(r.data)
	r.data = r.data[n:]
	return n, nil
}

func (r *dataResource) Write(_ contracts.CallContext, from abi.ByteView) (int, error) {
	if r.released != 0 {
		return 0, syscall.EBADF
	}
	data := make([]byte, from.Len())
	n := from.Read(data)
	r.data = append(r.data, data...)
	return n, nil
}

func (r *dataResource) Subscribe(ctx contracts.CallContext, sink contracts.ReadinessSink) error {
	if r.sinks == nil {
		r.sinks = make(map[contracts.ReadinessSink]bool)
	}
	r.epoch = ctx.Epoch
	r.sinks[sink] = true
	sink.Notify(ctx.Epoch, r.fd, r.ready)
	if r.onSubscribe != nil {
		r.onSubscribe(ctx, sink)
	}
	return r.subscribeErr
}

func (r *dataResource) Unsubscribe(_ contracts.CallContext, sink contracts.ReadinessSink) {
	delete(r.sinks, sink)
}

func (r *dataResource) notify(ready contracts.Readiness) {
	r.ready = ready
	for sink := range r.sinks {
		sink.Notify(r.epoch, r.fd, ready)
	}
}

type fcntlFiles struct{ contracts.FileOps }

func (*fcntlFiles) FcntlInt(_ contracts.CallContext, fd, command, arg int) (int, error) {
	return 1000 + fd + command + arg, nil
}

type fcntlSockets struct{ contracts.SocketOps }

func (*fcntlSockets) FcntlInt(_ contracts.CallContext, fd, command, arg int) (int, error) {
	return 2000 + fd + command + arg, nil
}
