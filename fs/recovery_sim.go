//go:build sim

package fs

import (
	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func CrashStates(m gosim.Machine, opts EnumerateOptions) (*CrashIterator, error) {
	r, err := control.Call(
		kernel.FSEnumerateRequest{Machine: ref(m), MaxStates: opts.MaxStates},
		kernel.DecodeFSEnumerateResult,
	)
	if err != nil {
		return nil, err
	}
	return &CrashIterator{m.RunID(), r.Iterator}, nil
}

func (i *CrashIterator) Next() (Snapshot, bool, error) {
	if err := control.Require(contracts.FS, "fs.CrashIterator.Next"); err != nil {
		return Snapshot{}, false, err
	}
	if i == nil {
		return Snapshot{}, false, gosim.ErrInvalidConfig
	}
	r, err := control.Call(
		kernel.FSNextSnapshotRequest{RunID: i.runID, Iterator: i.iteratorID},
		kernel.DecodeFSNextSnapshotResult,
	)
	if err != nil || !r.OK {
		return Snapshot{}, false, err
	}
	return Snapshot{i.runID, r.Snapshot}, true, nil
}

func (i *CrashIterator) Close() error {
	if err := control.Require(contracts.FS, "fs.CrashIterator.Close"); err != nil {
		return err
	}
	if i == nil {
		return gosim.ErrInvalidConfig
	}
	_, err := control.Call(
		kernel.FSCloseIteratorRequest{RunID: i.runID, Iterator: i.iteratorID},
		kernel.DecodeFSCloseIteratorResult,
	)
	return err
}

func Restore(m gosim.Machine, s Snapshot) error {
	_, err := control.Call(
		kernel.FSRestoreRequest{Machine: ref(m), SnapshotRunID: s.runID, Snapshot: s.snapshotID},
		kernel.DecodeFSRestoreResult,
	)
	return err
}

func (s Snapshot) Close() error {
	_, err := control.Call(
		kernel.FSCloseSnapshotRequest{RunID: s.runID, Snapshot: s.snapshotID},
		kernel.DecodeFSCloseSnapshotResult,
	)
	return err
}
