package fs

import (
	"errors"
	"math"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
)

func crashWithPending(t *testing.T, s *Session) {
	t.Helper()
	i := start(t, s, ref1, 1, false)
	write(t, i.model, "/app/recover", "retained")
	stop(t, i, contracts.StopCrash)
}

func enumerate(t *testing.T, s *Session, ref contracts.MachineRef, limit uint64) uint64 {
	t.Helper()
	id, err := s.Enumerate(ref, limit)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func next(t *testing.T, s *Session, id uint64) uint64 {
	t.Helper()
	snapshot, ok, err := s.NextSnapshot(id)
	if err != nil || !ok {
		t.Fatal(snapshot, ok, err)
	}
	return snapshot
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnumerationBudgetAndLookahead(t *testing.T) {
	s, _ := newSession(t)
	crashWithPending(t, s)
	if _, err := s.Enumerate(ref1, 0); !errors.Is(err, contracts.ErrInvalidConfig) {
		t.Fatal(err)
	}
	id := enumerate(t, s, ref1, 1)
	snapshot := next(t, s, id)
	if _, ok, err := s.NextSnapshot(id); ok || !errors.Is(err, contracts.ErrEnumerationLimit) {
		t.Fatal("budget was not distinguished from exhaustion", ok, err)
	}
	iterator := s.handles[id].iterator
	lookahead := iterator.lookahead
	if lookahead == nil {
		t.Fatal("missing bounded lookahead")
	}
	if _, _, err := s.NextSnapshot(id); !errors.Is(err, contracts.ErrEnumerationLimit) || iterator.lookahead != lookahead {
		t.Fatal("budget check advanced traversal")
	}
	must(t, s.CloseIterator(id))
	must(t, s.CloseIterator(id))
	if iterator.lookahead != nil || iterator.model != nil {
		t.Fatal("closed iterator retained resources")
	}
	target := contracts.MachineRef{RunID: 1, ID: 2}
	must(t, s.Restore(target, snapshot))
	must(t, s.CloseSnapshot(snapshot))
	must(t, s.CloseSnapshot(snapshot))
	i := start(t, s, target, 1, false)
	if read(t, i.model, "/app/recover") != "retained" {
		t.Fatal("closing handles invalidated restored disk")
	}
	stop(t, i, contracts.StopGraceful)
}

func TestExactBudgetIsExhaustion(t *testing.T) {
	s, _ := newSession(t)
	i := start(t, s, ref1, 1, false)
	stop(t, i, contracts.StopCrash)
	id := enumerate(t, s, ref1, 1)
	snapshot := next(t, s, id)
	for range 2 {
		if _, ok, err := s.NextSnapshot(id); err != nil || ok {
			t.Fatal("exact budget falsely reported limit", ok, err)
		}
	}
	must(t, s.CloseSnapshot(snapshot))
	must(t, s.CloseIterator(id))
}

func TestReturnedSnapshotsSurviveDestroyAndOtherRestores(t *testing.T) {
	s, _ := newSession(t)
	crashWithPending(t, s)
	id := enumerate(t, s, ref1, 100)
	snapshot := next(t, s, id)
	image := s.handles[snapshot].snapshot
	target := contracts.MachineRef{RunID: 1, ID: 2}
	must(t, s.Restore(target, snapshot))
	s.Destroy(ref1)
	if s.handles[id].iterator != nil {
		t.Fatal("destroy did not close source iterator")
	}
	must(t, s.CloseIterator(id))
	if _, _, err := s.NextSnapshot(id); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("destroyed iterator was usable", err)
	}
	other := contracts.MachineRef{RunID: 1, ID: 3}
	must(t, s.Restore(other, snapshot))
	must(t, s.CloseSnapshot(snapshot))
	// Closing the public snapshot releases its reference, but both targets own
	// independent chunks. Starting/writing one cannot change the other.
	a := start(t, s, target, 1, false)
	write(t, a.model, "/app/recover", "modified")
	stop(t, a, contracts.StopGraceful)
	b := start(t, s, other, 1, false)
	if read(t, b.model, "/app/recover") != "retained" {
		t.Fatal("restore shared mutable state")
	}
	stop(t, b, contracts.StopGraceful)
	if s.handles[snapshot].snapshot != nil {
		t.Fatal("closed snapshot retained payload")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("closed image still usable")
		}
	}()
	image.Clone(false, nil)
}

func TestRestoreAtomicAndRetainedAcrossFailedPrepare(t *testing.T) {
	s, _ := newSession(t)
	crashWithPending(t, s)
	id := enumerate(t, s, ref1, 100)
	snapshot := next(t, s, id)
	target := contracts.MachineRef{RunID: 1, ID: 2}
	must(t, s.Restore(target, snapshot))
	disk := s.machines[target.ID].disk
	must(t, s.CloseSnapshot(snapshot))
	if err := s.Restore(target, snapshot); !errors.Is(err, contracts.ErrInvalidState) || s.machines[target.ID].disk != disk {
		t.Fatal("invalid restore changed disk")
	}
	p := prepare(t, s, target, 1, false)
	p.Abort()
	if s.machines[target.ID].disk != disk {
		t.Fatal("prepare abort replaced restored disk")
	}
	i := start(t, s, target, 2, false)
	if read(t, i.model, "/app/recover") != "retained" {
		t.Fatal("restore lost after handle close/abort")
	}
	if err := s.Restore(target, 0); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal(err)
	}
	stop(t, i, contracts.StopCrash)
	must(t, s.CloseIterator(id))
}

func TestRecoveryIDsAndSessionCleanup(t *testing.T) {
	s, _ := newSession(t)
	crashWithPending(t, s)
	id := enumerate(t, s, ref1, 1)
	snapshot := next(t, s, id)
	if id == snapshot || id == 0 {
		t.Fatal("handle IDs not distinct")
	}
	if err := s.CloseSnapshot(id); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("wrong kind accepted", err)
	}
	if err := s.CloseIterator(snapshot); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("wrong kind accepted", err)
	}
	if err := s.CloseSnapshot(999); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("unknown handle accepted", err)
	}
	s.nextHandle = math.MaxUint64
	before := len(s.handles)
	if _, err := s.Enumerate(ref1, 1); err == nil || len(s.handles) != before {
		t.Fatal("exhausted handle allocation leaked or succeeded")
	}
	_, _, _ = s.NextSnapshot(id) // retain one lookahead at the budget boundary
	iterator := s.handles[id].iterator
	image := s.handles[snapshot].snapshot
	machine := s.machines[ref1.ID]
	s.Close()
	s.Close()
	if iterator.lookahead != nil || iterator.model != nil || machine.disk != nil || machine.crash != nil || len(s.handles) != 0 || len(s.machines) != 0 {
		t.Fatal("session close retained resources")
	}
	if _, _, err := s.NextSnapshot(id); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("session close did not release snapshot")
		}
	}()
	image.Clone(false, nil)
}

func TestNextSnapshotAllocationFailureDoesNotAdvance(t *testing.T) {
	s, _ := newSession(t)
	crashWithPending(t, s)
	id := enumerate(t, s, ref1, 100)
	s.nextHandle = math.MaxUint64
	if _, _, err := s.NextSnapshot(id); err == nil {
		t.Fatal("ID exhaustion accepted")
	}
	it := s.handles[id].iterator
	if it.yielded != 0 || it.lookahead == nil {
		t.Fatal("failed allocation consumed a state")
	}
	must(t, s.CloseIterator(id))
	if it.lookahead != nil || it.model != nil {
		t.Fatal("failure cleanup retained lookahead")
	}
}

func TestSessionCloseReleasesStagedAndActiveModels(t *testing.T) {
	s, _ := newSession(t)
	staged := prepare(t, s, ref1, 1, false)
	active := start(t, s, contracts.MachineRef{RunID: 1, ID: 2}, 1, false)
	write(t, active.model, "/app/unsynced", "data")
	s.Close()
	if staged.instance.model != nil || active.model != nil || !staged.aborted || !active.detached || !active.quiesced {
		t.Fatal("close retained epoch state")
	}
}

func TestFailedRestartKeepsCrashImageAndIterators(t *testing.T) {
	s, _ := newSession(t)
	crashWithPending(t, s)
	id := enumerate(t, s, ref1, 100)
	image, disk := s.machines[1].crash, s.machines[1].disk
	p := prepare(t, s, ref1, 2, true)
	write(t, p.instance.model, "/app/recover", "aborted")
	p.Abort()
	if s.machines[1].crash != image || s.machines[1].disk != disk {
		t.Fatal("aborted restart lost crash state")
	}
	snapshot := next(t, s, id)
	i := start(t, s, ref1, 3, true)
	if s.machines[1].crash != nil {
		t.Fatal("new epoch retained obsolete machine crash image")
	}
	// An iterator owns the old image independently of subsequent machine epochs.
	if _, ok, err := s.NextSnapshot(id); err != nil || !ok {
		t.Fatal("restart invalidated existing iterator", ok, err)
	}
	stop(t, i, contracts.StopGraceful)
	if _, err := s.Enumerate(ref1, 10); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("graceful stop exposed old crash image", err)
	}
	target := contracts.MachineRef{RunID: 1, ID: 2}
	must(t, s.Restore(target, snapshot))
	must(t, s.CloseSnapshot(snapshot))
	must(t, s.CloseIterator(id))
	i = start(t, s, target, 1, false)
	if read(t, i.model, "/app/recover") != "retained" {
		t.Fatal("old snapshot changed after restart")
	}
	stop(t, i, contracts.StopCrash)
}

func TestPrepareMissingRestoredWorkingDirectoryIsAtomic(t *testing.T) {
	s, _ := newSession(t)
	target := contracts.MachineRef{RunID: 1, ID: 2}
	must(t, s.ConfigureMachine(target, contracts.FSConfig{WorkingDir: "/etc"}))
	i := start(t, s, ref1, 1, false)
	must(t, i.model.Remove(1, "/etc/hosts", false))
	must(t, i.model.Remove(1, "/etc", true))
	stop(t, i, contracts.StopCrash)
	id := enumerate(t, s, ref1, 100)
	snapshot := next(t, s, id)
	must(t, s.Restore(target, snapshot))
	disk := s.machines[target.ID].disk
	_, err := s.Prepare(contracts.StartSpec{Ref: contracts.EpochRef{Machine: target, Epoch: 1}}, noDescriptors{})
	if err == nil || s.machines[target.ID].disk != disk || s.machines[target.ID].prepared != nil || s.machines[target.ID].started {
		t.Fatal("failed working directory resolution published state", err)
	}
	must(t, s.ConfigureMachine(target, contracts.FSConfig{WorkingDir: "/app"}))
	i = start(t, s, target, 2, false)
	stop(t, i, contracts.StopGraceful)
}
