package contracts

import (
	"errors"
	"slices"
)

var errInjected = errors.New("injected contract failure")

type fakeEvent struct {
	epoch EpochRef
	run   func()
}

type fakePorts struct {
	now, draws uint64
	next       EventID
	events     map[EventID]fakeEvent
	fields     []Field
}

func (p *fakePorts) ports() Ports {
	return Ports{Clock: p, Events: p, Trace: p, RandomFor: func(Feature, MachineRef) Random { return p }}
}

func (p *fakePorts) NowNanos() int64         { return int64(p.now) }
func (p *fakePorts) Uint64() uint64          { p.draws++; return p.draws }
func (p *fakePorts) Uint64N(n uint64) uint64 { return p.Uint64() % n }
func (p *fakePorts) Float64() float64        { return 0.5 }
func (p *fakePorts) ExpFloat64() float64     { return 1 }
func (p *fakePorts) Schedule(epoch EpochRef, _ int64, callback func()) (EventID, error) {
	if p.events == nil {
		p.events = make(map[EventID]fakeEvent)
	}
	p.next++
	p.events[p.next] = fakeEvent{epoch: epoch, run: callback}
	return p.next, nil
}
func (p *fakePorts) Cancel(id EventID) { delete(p.events, id) }
func (p *fakePorts) CancelEpoch(epoch EpochRef) {
	for id, event := range p.events {
		if event.epoch == epoch {
			delete(p.events, id)
		}
	}
}
func (p *fakePorts) Emit(_ EpochRef, _ string, fields []Field) { p.fields = slices.Clone(fields) }

type fakeProvider struct {
	feature               Feature
	log                   *[]string
	failOpen, failPrepare bool
}

type (
	fakeSession  struct{ provider *fakeProvider }
	fakePrepared struct{ session *fakeSession }
	fakeInstance struct{ session *fakeSession }
)

func (p *fakeProvider) Feature() Feature { return p.feature }
func (p *fakeProvider) Open(ports Ports) (ProviderSession, error) {
	*p.log = append(*p.log, "open")
	if err := ValidatePorts(ports); err != nil {
		return nil, err
	}
	if p.failOpen {
		return nil, errInjected
	}
	return &fakeSession{provider: p}, nil
}

func (s *fakeSession) Controls() ControlBindings {
	if s.provider.feature == FS {
		return ControlBindings{FSControl: &fakeFSControl{}}
	}
	return ControlBindings{NetControl: &fakeNetControl{}}
}

func (s *fakeSession) Prepare(StartSpec, DescriptorTable) (PreparedMachine, error) {
	*s.provider.log = append(*s.provider.log, "prepare")
	if s.provider.failPrepare {
		return nil, errInjected
	}
	return &fakePrepared{session: s}, nil
}
func (s *fakeSession) Destroy(MachineRef) { *s.provider.log = append(*s.provider.log, "destroy") }
func (s *fakeSession) Close()             { *s.provider.log = append(*s.provider.log, "close") }
func (p *fakePrepared) Bindings() Bindings {
	if p.session.provider.feature == FS {
		return Bindings{Files: &fakeFiles{}}
	}
	return Bindings{Sockets: &fakeSockets{}}
}

func (p *fakePrepared) Publish() MachineInstance {
	*p.session.provider.log = append(*p.session.provider.log, "publish")
	return &fakeInstance{session: p.session}
}
func (p *fakePrepared) Abort() { *p.session.provider.log = append(*p.session.provider.log, "abort") }
func (i *fakeInstance) Quiesce() {
	*i.session.provider.log = append(*i.session.provider.log, "quiesce")
}

func (i *fakeInstance) Settle(StopKind) error {
	*i.session.provider.log = append(*i.session.provider.log, "settle")
	return nil
}
func (i *fakeInstance) Detach() { *i.session.provider.log = append(*i.session.provider.log, "detach") }

var (
	_ Provider        = (*fakeProvider)(nil)
	_ ProviderSession = (*fakeSession)(nil)
	_ PreparedMachine = (*fakePrepared)(nil)
	_ MachineInstance = (*fakeInstance)(nil)
	_ Clock           = (*fakePorts)(nil)
	_ Random          = (*fakePorts)(nil)
	_ Events          = (*fakePorts)(nil)
	_ Trace           = (*fakePorts)(nil)
)
