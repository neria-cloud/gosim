package net

import (
	"slices"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/net/model"
)

type Session struct {
	ports     contracts.Ports
	network   *model.Network
	runID     uint64
	machines  map[uint64]*machine
	destroyed map[uint64]bool
	closed    bool
}
type machine struct {
	ref        contracts.MachineRef
	configured [4]byte
	automatic  bool
	address    [4]byte
	started    bool
	prepared   *Prepared
	active     *Instance
}

func (s *Session) Controls() contracts.ControlBindings {
	return contracts.ControlBindings{NetControl: s}
}

func (s *Session) checkRef(ref contracts.MachineRef) error {
	if s.closed {
		return contracts.ErrInvalidState
	}
	if ref.RunID == 0 || s.runID != 0 && s.runID != ref.RunID {
		return contracts.ErrWrongRun
	}
	if ref.ID == 0 || s.destroyed[ref.ID] {
		return contracts.ErrMachineNotFound
	}
	return nil
}

func (s *Session) machine(ref contracts.MachineRef) *machine {
	if m := s.machines[ref.ID]; m != nil {
		return m
	}
	s.runID = ref.RunID
	m := &machine{ref: ref, automatic: true}
	s.machines[ref.ID] = m
	return m
}

func (s *Session) Prepare(spec contracts.StartSpec, fds contracts.DescriptorTable) (contracts.PreparedMachine, error) {
	if err := s.checkRef(spec.Ref.Machine); err != nil {
		return nil, err
	}
	if spec.Ref.Epoch == 0 || fds == nil {
		return nil, contracts.ErrInvalidConfig
	}
	m := s.machines[spec.Ref.Machine.ID]
	if m != nil && (m.active != nil || m.prepared != nil || m.started != spec.Restart) || m == nil && spec.Restart {
		return nil, contracts.ErrInvalidState
	}
	m = s.machine(spec.Ref.Machine)
	reservation, err := s.network.ReserveAddress(m.ref, m.configured, !m.automatic)
	if err != nil {
		return nil, err
	}
	i := &Instance{session: s, machine: m, ref: spec.Ref, fds: fds, stack: model.NewStack(spec.Ref, s.ports), sockets: make(map[int]*Socket)}
	p := &Prepared{instance: i, reservation: reservation}
	m.prepared = p
	return p, nil
}

func (s *Session) Destroy(ref contracts.MachineRef) {
	if s.checkRef(ref) != nil {
		return
	}
	if s.runID == 0 {
		s.runID = ref.RunID
	}
	if m := s.machines[ref.ID]; m != nil {
		if m.prepared != nil {
			m.prepared.Abort()
		}
		if m.active != nil {
			m.active.Quiesce()
			m.active.Detach()
		}
	}
	s.network.ReleaseAddress(ref)
	delete(s.machines, ref.ID)
	s.destroyed[ref.ID] = true
}

func (s *Session) Close() {
	if s.closed {
		return
	}
	ids := make([]uint64, 0, len(s.machines))
	for id := range s.machines {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		s.Destroy(s.machines[id].ref)
	}
	s.network.Close()
	s.network = nil
	s.machines = nil
	s.destroyed = nil
	s.ports = contracts.Ports{}
	s.closed = true
}
