package fs

import (
	"fmt"
	"math"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/fs/model"
)

func validateConfig(config contracts.FSConfig) error     { return contracts.ValidateFSConfig(config) }
func validateFault(config contracts.FSFaultConfig) error { return contracts.ValidateFSFault(config) }
func (s *Session) ConfigureMachine(ref contracts.MachineRef, config contracts.FSConfig) error {
	// TODO(not planned): filesystem resize is outside the configured startup capacity contract.
	if err := s.checkRef(ref); err != nil {
		return err
	}
	if err := validateConfig(config); err != nil {
		return err
	}
	if config.CapacityBytes == 0 {
		config.CapacityBytes = model.DefaultCapacityBytes
	}
	m := s.machines[ref.ID]
	if m != nil && (m.started || m.prepared != nil || m.active != nil) {
		return contracts.ErrInvalidState
	}
	// Validate the directory against the next-start disk before changing config.
	var disk *model.Filesystem
	if m != nil {
		disk = m.disk
	}
	if disk == nil {
		disk = model.NewLinuxFilesystem()
		defer disk.Release()
	}
	if _, err := disk.Getdirinode(model.RootInode, config.WorkingDir); err != nil {
		return err
	}
	s.machine(ref).config = config
	return nil
}

func (s *Session) SetReadOnly(ref contracts.MachineRef, enabled bool) error {
	if err := s.checkRef(ref); err != nil {
		return err
	}
	s.machine(ref).readOnly = enabled
	return nil
}

func (s *Session) Stats(ref contracts.MachineRef) (contracts.FSStorageStats, error) {
	if err := s.checkRef(ref); err != nil {
		return contracts.FSStorageStats{}, err
	}
	m := s.machine(ref)
	disk := m.disk
	if m.active != nil {
		disk = m.active.model
	}
	if disk == nil {
		disk = model.NewLinuxFilesystem()
		defer disk.Release()
	}
	stats := disk.Stats(m.config.CapacityBytes)
	return contracts.FSStorageStats{TotalBytes: stats.TotalBytes, FreeBytes: stats.FreeBytes, UsedBytes: stats.UsedBytes, BlockSize: stats.BlockSize, ReadOnly: m.readOnly}, nil
}

func (s *Session) SetCrashMode(ref contracts.MachineRef, mode uint8) error {
	if err := s.checkRef(ref); err != nil {
		return err
	}
	if mode > 1 {
		return fmt.Errorf("%w: invalid crash mode", contracts.ErrInvalidConfig)
	}
	s.machine(ref).crashMode = mode
	return nil
}

func (s *Session) SetSyncFault(ref contracts.MachineRef, fault contracts.FSFaultConfig) error {
	if err := s.checkRef(ref); err != nil {
		return err
	}
	if err := validateFault(fault); err != nil {
		return err
	}
	s.machine(ref).fault = fault
	return nil
}

func (s *Session) InspectInode(ref contracts.MachineRef, inode int) (contracts.InodeInfo, error) {
	if err := s.checkRef(ref); err != nil {
		return contracts.InodeInfo{}, err
	}
	if inode <= 0 {
		return contracts.InodeInfo{}, contracts.ErrInvalidConfig
	}
	m := s.machines[ref.ID]
	if m == nil {
		return contracts.InodeInfo{}, contracts.ErrNotStarted
	}
	disk := m.disk
	if m.active != nil {
		disk = m.active.model
	}
	if disk == nil {
		return contracts.InodeInfo{}, contracts.ErrNotStarted
	}
	info := disk.GetInodeInfo(inode)
	return contracts.InodeInfo{MemLinks: info.MemLinks, MemExists: info.MemExists, DiskLinks: info.DiskLinks, DiskExists: info.DiskExists, Handles: info.Handles, Ops: info.Ops}, nil
}

type recoveryHandle struct {
	kind     uint8 // 1=iterator, 2=snapshot; nil payload is an issued, closed handle
	iterator *recoveryIterator
	snapshot *model.CrashImage
}
type recoveryIterator struct {
	owner          contracts.MachineRef
	model          *model.CrashIterator
	lookahead      *model.Filesystem
	limit, yielded uint64
	exhausted      bool
}

func (s *Session) allocateHandle(h *recoveryHandle) (uint64, error) {
	if s.nextHandle == 0 || s.nextHandle == math.MaxUint64 {
		return 0, &contracts.Failure{Code: contracts.InvariantFailure, Diagnostic: contracts.Diagnostic{Feature: "fs", Operation: "allocate recovery handle", Message: "handle ID space exhausted"}}
	}
	id := s.nextHandle
	s.nextHandle++
	s.handles[id] = h
	return id, nil
}

func (s *Session) lookupHandle(id uint64, kind uint8) (*recoveryHandle, error) {
	if s.closed {
		return nil, contracts.ErrInvalidState
	}
	h := s.handles[id]
	if h == nil || h.kind != kind {
		return nil, contracts.ErrInvalidState
	}
	return h, nil
}

func (s *Session) Enumerate(ref contracts.MachineRef, maxStates uint64) (uint64, error) {
	if err := s.checkRef(ref); err != nil {
		return 0, err
	}
	if maxStates == 0 {
		return 0, contracts.ErrInvalidConfig
	}
	m := s.machines[ref.ID]
	if m == nil || m.crash == nil || m.active != nil || m.prepared != nil {
		return 0, contracts.ErrInvalidState
	}
	it := &recoveryIterator{owner: ref, model: m.crash.IterCrashes(), limit: maxStates}
	id, err := s.allocateHandle(&recoveryHandle{kind: 1, iterator: it})
	if err != nil {
		it.model.Close()
		return 0, err
	}
	return id, nil
}

func (s *Session) NextSnapshot(id uint64) (uint64, bool, error) {
	h, err := s.lookupHandle(id, 1)
	if err != nil {
		return 0, false, err
	}
	it := h.iterator
	if it == nil {
		return 0, false, contracts.ErrInvalidState
	}
	if it.exhausted {
		return 0, false, nil
	}
	if it.lookahead == nil {
		var ok bool
		it.lookahead, ok = it.model.Next()
		if !ok {
			it.exhausted = true
			return 0, false, nil
		}
	}
	if it.yielded == it.limit {
		return 0, false, contracts.ErrEnumerationLimit
	}
	image := it.lookahead.FreezeCrash()
	snapshotID, err := s.allocateHandle(&recoveryHandle{kind: 2, snapshot: image})
	if err != nil {
		image.Release()
		return 0, false, err
	}
	it.lookahead.Release()
	it.lookahead = nil
	it.yielded++
	return snapshotID, true, nil
}

func (s *Session) closeIterator(h *recoveryHandle) {
	if h.iterator == nil {
		return
	}
	it := h.iterator
	it.model.Close()
	if it.lookahead != nil {
		it.lookahead.Release()
		it.lookahead = nil
	}
	it.model = nil
	h.iterator = nil
}

func (s *Session) CloseIterator(id uint64) error {
	h, err := s.lookupHandle(id, 1)
	if err != nil {
		return err
	}
	s.closeIterator(h)
	return nil
}

func (s *Session) Restore(ref contracts.MachineRef, id uint64) error {
	if err := s.checkRef(ref); err != nil {
		return err
	}
	h, err := s.lookupHandle(id, 2)
	if err != nil {
		return err
	}
	if h.snapshot == nil {
		return contracts.ErrInvalidState
	}
	m := s.machines[ref.ID]
	if m != nil && (m.prepared != nil || m.active != nil) {
		return contracts.ErrInvalidState
	}
	disk := h.snapshot.Clone(false, nil)
	m = s.machine(ref)
	if m.disk != nil {
		m.disk.Release()
	}
	m.disk = disk
	// Restore replaces only the next-start disk. A crashed source's immutable
	// pre-crash image and already-open enumerations remain independently owned.
	return nil
}

func (s *Session) CloseSnapshot(id uint64) error {
	h, err := s.lookupHandle(id, 2)
	if err != nil {
		return err
	}
	if h.snapshot != nil {
		h.snapshot.Release()
		h.snapshot = nil
	}
	return nil
}
