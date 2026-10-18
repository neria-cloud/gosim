package contracts

import (
	"errors"
	"testing"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// These recording fakes exercise the interface boundary, not fd allocation or
// polling policy; those are kernel responsibilities.
type fakeResource struct {
	owner   Feature
	data    []byte
	reasons []CloseReason
	sink    ReadinessSink
}

func (r *fakeResource) Owner() Feature { return r.owner }
func (r *fakeResource) Release(reason CloseReason) error {
	r.reasons = append(r.reasons, reason)
	return errInjected
}

func (r *fakeResource) Read(_ CallContext, view abi.ByteView) (int, error) {
	return view.Write(r.data), nil
}

func (r *fakeResource) Write(_ CallContext, view abi.ByteView) (int, error) {
	r.data = make([]byte, view.Len())
	return view.Read(r.data), nil
}

func (r *fakeResource) Subscribe(_ CallContext, sink ReadinessSink) error {
	r.sink = sink
	return nil
}
func (r *fakeResource) Unsubscribe(_ CallContext, _ ReadinessSink) { r.sink = nil }

type fakeDescriptors struct{ resource Resource }

func (f *fakeDescriptors) Allocate(r Resource) (int, error) { f.resource = r; return 3, nil }
func (f *fakeDescriptors) Lookup(fd int) (Resource, error) {
	if fd != 3 || f.resource == nil {
		return nil, errInjected
	}
	return f.resource, nil
}

func (f *fakeDescriptors) Remove(fd int) (Resource, error) {
	r, err := f.Lookup(fd)
	if err == nil {
		f.resource = nil
	}
	return r, err
}

type fakeReadiness struct {
	epoch EpochRef
	fd    int
	ready Readiness
}

func (s *fakeReadiness) Notify(epoch EpochRef, fd int, ready Readiness) {
	s.epoch, s.fd, s.ready = epoch, fd, ready
}

var (
	_ Resource        = (*fakeResource)(nil)
	_ Readable        = (*fakeResource)(nil)
	_ Writable        = (*fakeResource)(nil)
	_ Pollable        = (*fakeResource)(nil)
	_ DescriptorTable = (*fakeDescriptors)(nil)
	_ ReadinessSink   = (*fakeReadiness)(nil)
)

func TestDescriptorCapabilityContracts(t *testing.T) {
	resource := &fakeResource{owner: Net}
	var table DescriptorTable = &fakeDescriptors{}
	fd, err := table.Allocate(resource)
	if err != nil {
		t.Fatal(err)
	}
	r, err := table.Lookup(fd)
	if err != nil || r.Owner() != Net {
		t.Fatal("resource ownership lost")
	}
	call := CallContext{Epoch: EpochRef{Machine: MachineRef{RunID: 2, ID: 3}, Epoch: 4}}
	input := []byte("data")
	if n, err := r.(Writable).Write(call, abi.ByteView{Ptr: input}); n != len(input) || err != nil {
		t.Fatal(n, err)
	}
	input[0] = 'x'
	output := make([]byte, 4)
	if n, err := r.(Readable).Read(call, abi.ByteView{Ptr: output}); n != 4 || err != nil || string(output) != "data" {
		t.Fatal(n, err, string(output))
	}
	sink := &fakeReadiness{}
	if err := r.(Pollable).Subscribe(call, sink); err != nil {
		t.Fatal(err)
	}
	resource.sink.Notify(call.Epoch, fd, ReadReady|Hangup)
	if sink.epoch != call.Epoch || sink.fd != fd || sink.ready != ReadReady|Hangup {
		t.Fatal("readiness value changed")
	}
	r.(Pollable).Unsubscribe(call, sink)
	if resource.sink != nil {
		t.Fatal("subscription retained")
	}
	r, err = table.Remove(fd)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Release(CloseCrash); !errors.Is(err, errInjected) || resource.reasons[0] != CloseCrash {
		t.Fatal("release contract lost")
	}
	if _, err := table.Lookup(fd); err == nil {
		t.Fatal("removed resource retained")
	}
}
