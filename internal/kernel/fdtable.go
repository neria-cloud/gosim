package kernel

import (
	"errors"
	"math"
	"slices"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
)

// TODO(migration): complete descriptor capabilities and poll ownership in Task 6.
// This table already owns staged resources and performs real abort/stop cleanup.
type FDTable struct {
	epoch   contracts.EpochRef
	entries map[int]contracts.Resource
	next    int
	closed  bool
}

func NewFDTable(epoch contracts.EpochRef) *FDTable {
	return &FDTable{epoch: epoch, entries: make(map[int]contracts.Resource), next: 3}
}

func (t *FDTable) Allocate(resource contracts.Resource) (int, error) {
	if t.closed || resource == nil {
		return 0, contracts.ErrInvalidState
	}
	if t.next > math.MaxInt32 {
		return 0, syscall.EMFILE
	}
	fd := t.next
	t.next++
	t.entries[fd] = resource
	return fd, nil
}

func (t *FDTable) Lookup(fd int) (contracts.Resource, error) {
	resource := t.entries[fd]
	if resource == nil {
		return nil, syscall.EBADF
	}
	return resource, nil
}

func (t *FDTable) Remove(fd int) (contracts.Resource, error) {
	resource, err := t.Lookup(fd)
	if err != nil {
		return nil, err
	}
	delete(t.entries, fd)
	return resource, nil
}

func (t *FDTable) Finalize(reason contracts.CloseReason) error {
	if t.closed {
		return nil
	}
	t.closed = true
	var fds []int
	for fd := range t.entries {
		fds = append(fds, fd)
	}
	slices.Sort(fds)
	var failures []error
	for _, fd := range fds {
		resource := t.entries[fd]
		delete(t.entries, fd)
		failures = append(failures, protected(func() error { return resource.Release(reason) }))
	}
	return errors.Join(failures...)
}

type diagnosticResource struct{ closed bool }

func (*diagnosticResource) Owner() contracts.Feature              { return contracts.RT }
func (r *diagnosticResource) Release(contracts.CloseReason) error { r.closed = true; return nil }

func (t *FDTable) installDiagnostics() error {
	if t.closed || len(t.entries) != 0 {
		return contracts.ErrInvalidState
	}
	for fd := 0; fd < 3; fd++ {
		t.entries[fd] = &diagnosticResource{}
	}
	return nil
}
