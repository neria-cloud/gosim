package fs

import (
	"slices"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/fs/model"
)

type Session struct {
	flockSemantics FlockSemantics
	ports          contracts.Ports
	runID          uint64
	machines       map[uint64]*machine
	destroyed      map[uint64]bool
	handles        map[uint64]*recoveryHandle
	nextHandle     uint64
	closed         bool
}
type machine struct {
	ref       contracts.MachineRef
	config    contracts.FSConfig
	fault     contracts.FSFaultConfig
	crashMode uint8
	readOnly  bool
	started   bool
	disk      *model.Filesystem
	crash     *model.CrashImage
	prepared  *Prepared
	active    *Instance
}

func (s *Session) Controls() contracts.ControlBindings {
	return contracts.ControlBindings{FSControl: s}
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

// The kernel validates machine existence and state before forwarding. A session
// first learns of an unstarted machine through a control or Prepare call.
func (s *Session) machine(ref contracts.MachineRef) *machine {
	if m := s.machines[ref.ID]; m != nil {
		return m
	}
	s.runID = ref.RunID
	m := &machine{ref: ref, config: contracts.FSConfig{WorkingDir: "/app", CapacityBytes: model.DefaultCapacityBytes}}
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
	var disk *model.Filesystem
	if m.disk == nil {
		disk = model.NewLinuxFilesystem()
	} else {
		disk = cloneDisk(m.disk)
	}
	cwd, err := disk.Getdirinode(model.RootInode, m.config.WorkingDir, model.Identity{UID: m.config.UID, GID: m.config.GID})
	if err != nil {
		disk.Release()
		return nil, err
	}
	instance := &Instance{session: s, machine: m, ref: spec.Ref, model: disk, fds: fds, workingDir: cwd}
	prepared := &Prepared{instance: instance}
	m.prepared = prepared
	return prepared, nil
}

func cloneDisk(disk *model.Filesystem) *model.Filesystem {
	image := disk.FreezeCrash()
	defer image.Release()
	return image.Clone(false, nil)
}

func (s *Session) Destroy(ref contracts.MachineRef) {
	if s.checkRef(ref) != nil {
		return
	}
	if s.runID == 0 {
		s.runID = ref.RunID
	}
	var cleanup cleanupErrors
	m := s.machines[ref.ID]
	if m != nil {
		// Kernel destruction happens only after epoch cleanup. Close also uses this
		// path defensively to release partially staged/published state.
		if m.prepared != nil {
			cleanup.run(func() error { m.prepared.Abort(); return nil })
		}
		if m.active != nil {
			cleanup.run(func() error { m.active.Detach(); return nil })
		}
		if m.disk != nil {
			cleanup.run(func() error { m.disk.Release(); return nil })
			m.disk = nil
		}
		if m.crash != nil {
			cleanup.run(func() error { m.crash.Release(); return nil })
			m.crash = nil
		}
	}
	for _, id := range s.handleIDs() {
		h := s.handles[id]
		if h.iterator != nil && h.iterator.owner == ref {
			cleanup.run(func() error { s.closeIterator(h); return nil })
		}
	}
	delete(s.machines, ref.ID)
	s.destroyed[ref.ID] = true
	cleanup.finish()
}

func (s *Session) Close() {
	if s.closed {
		return
	}
	var cleanup cleanupErrors
	var ids []uint64
	for id := range s.machines {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		cleanup.run(func() error { s.Destroy(s.machines[id].ref); return nil })
	}
	for _, id := range s.handleIDs() {
		h := s.handles[id]
		cleanup.run(func() error { s.closeIterator(h); return nil })
		if h.snapshot != nil {
			cleanup.run(func() error { h.snapshot.Release(); return nil })
			h.snapshot = nil
		}
	}
	s.machines, s.destroyed, s.handles = nil, nil, nil
	s.ports = contracts.Ports{}
	s.closed = true
	cleanup.finish()
}

func (s *Session) handleIDs() []uint64 {
	var ids []uint64
	for id := range s.handles {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
