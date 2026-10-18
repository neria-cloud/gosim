package kernel

import (
	"errors"
	"fmt"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

var errInjected = errors.New("injected provider failure")

type recordingProvider struct {
	feature                                             contracts.Feature
	log                                                 *[]string
	openErr, prepareErr, settleErr, releaseErr          error
	badControls, badBindings, panicPublish, nilInstance bool
	yieldPublish                                        bool
	panicQuiesce, panicDetach, panicAbort, panicClose   bool
	ports                                               contracts.Ports
	resources                                           []*recordingResource
}

func (p *recordingProvider) record(operation string) {
	*p.log = append(*p.log, featureName(p.feature)+":"+operation)
}
func (p *recordingProvider) Feature() contracts.Feature { return p.feature }
func (p *recordingProvider) Open(ports contracts.Ports) (contracts.ProviderSession, error) {
	p.record("open")
	p.ports = ports
	if p.openErr != nil {
		return nil, p.openErr
	}
	return &recordingSession{p}, nil
}

type (
	recordingSession struct{ p *recordingProvider }
	fakeFileOps      struct{ contracts.FileOps }
	fakeSocketOps    struct{ contracts.SocketOps }
	fakeFSControl    struct{ contracts.FSControl }
	fakeNetControl   struct{ contracts.NetControl }
)

func (s *recordingSession) Controls() contracts.ControlBindings {
	s.p.record("controls")
	if s.p.badControls {
		return contracts.ControlBindings{}
	}
	if s.p.feature == contracts.FS {
		return contracts.ControlBindings{FSControl: &fakeFSControl{}}
	}
	return contracts.ControlBindings{NetControl: &fakeNetControl{}}
}

func (s *recordingSession) Prepare(spec contracts.StartSpec, fds contracts.DescriptorTable) (contracts.PreparedMachine, error) {
	s.p.record(fmt.Sprintf("prepare:%d:%d", spec.Ref.Machine.ID, spec.Ref.Epoch))
	if s.p.prepareErr != nil {
		return nil, s.p.prepareErr
	}
	r := &recordingResource{p: s.p, epoch: spec.Ref, table: fds}
	fd, err := fds.Allocate(r)
	if err != nil {
		return nil, err
	}
	r.fd = fd
	s.p.resources = append(s.p.resources, r)
	return &recordingPrepared{p: s.p, resource: r}, nil
}

func (s *recordingSession) Destroy(ref contracts.MachineRef) {
	s.p.record(fmt.Sprintf("destroy:%d", ref.ID))
}

func (s *recordingSession) Close() {
	s.p.record("close")
	if s.p.panicClose {
		panic("close failure")
	}
}

type recordingPrepared struct {
	p        *recordingProvider
	resource *recordingResource
}

func (p *recordingPrepared) Bindings() contracts.Bindings {
	p.p.record("bindings")
	if p.p.badBindings {
		return contracts.Bindings{}
	}
	if p.p.feature == contracts.FS {
		return contracts.Bindings{Files: &fakeFileOps{}}
	}
	return contracts.Bindings{Sockets: &fakeSocketOps{}}
}

func (p *recordingPrepared) Publish() contracts.MachineInstance {
	p.p.record("publish")
	if p.p.yieldPublish {
		simrt.Yield()
	}
	if p.p.panicPublish {
		panic("publish failure")
	}
	if p.p.nilInstance {
		return nil
	}
	return &recordingInstance{p: p.p, resource: p.resource}
}

func (p *recordingPrepared) Abort() {
	p.p.record("abort")
	if !p.resource.released {
		panic("provider aborted before descriptors")
	}
	if p.p.panicAbort {
		panic("abort failure")
	}
}

type recordingInstance struct {
	p        *recordingProvider
	resource *recordingResource
	settled  bool
}

func (i *recordingInstance) Quiesce() {
	i.p.record("quiesce")
	if i.p.panicQuiesce {
		panic("quiesce failure")
	}
}

func (i *recordingInstance) Settle(kind contracts.StopKind) error {
	if i.settled {
		panic("settled twice")
	}
	i.settled = true
	i.p.record(fmt.Sprintf("settle:%d", kind))
	return i.p.settleErr
}

func (i *recordingInstance) Detach() {
	i.p.record("detach")
	if !i.resource.released {
		panic("detached before descriptor release")
	}
	if i.p.panicDetach {
		panic("detach failure")
	}
}

type recordingResource struct {
	p        *recordingProvider
	epoch    contracts.EpochRef
	table    contracts.DescriptorTable
	fd       int
	released bool
	reason   contracts.CloseReason
}

func (r *recordingResource) Owner() contracts.Feature { return r.p.feature }
func (r *recordingResource) Release(reason contracts.CloseReason) error {
	if r.released {
		panic("released twice")
	}
	if _, err := r.table.Lookup(r.fd); err == nil {
		panic("released before removing descriptor")
	}
	r.released, r.reason = true, reason
	r.p.record(fmt.Sprintf("release:%d:%d", r.epoch.Machine.ID, reason))
	return r.p.releaseErr
}
