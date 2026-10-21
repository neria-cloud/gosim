//go:build sim

package etcd_test

import (
	"errors"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"

	"github.com/neria-cloud/gosim/examples/etcd"
)

// TestSnapshotter smoke tests the low-level snap package in etcd.
func TestSnapshotter(t *testing.T) {
	snapdir := "snap"

	zapLogger := etcd.MakeZapLogger()

	// create a snapshotter
	t.Logf("creating snapshotter")
	if err := os.Mkdir(snapdir, 0o750); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	snapshotter := snap.New(zapLogger, snapdir)
	if _, err := snapshotter.Load(); !errors.Is(err, snap.ErrNoSnapshot) {
		t.Errorf("snapshotter.Load: expected %s, got %s", snap.ErrNoSnapshot, err)
	}

	// save some data
	t.Logf("writing snapshot")
	if err := snapshotter.SaveSnap(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:     3,
			ConfState: raftpb.ConfState{},
			Term:      1,
		},
		Data: []byte{1, 2, 3, 10, 13},
	}); err != nil {
		t.Fatalf("SaveSnap: %s", err)
	}

	// load it
	snapshot, err := snapshotter.Load()
	if err != nil {
		t.Fatalf("snapshotter.Load: %s", err)
	}
	t.Logf("read snapshot %s", snapshot)
	if diff := cmp.Diff(snapshot, &raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:     3,
			ConfState: raftpb.ConfState{},
			Term:      1,
		},
		Data: []byte{1, 2, 3, 10, 13},
	}); diff != "" {
		t.Errorf("loaded snapshot differs: %s", diff)
	}
}

// TestSnapshotter smoke tests the low-level wal package in etcd.
func TestWal(t *testing.T) {
	waldir := "wal"

	// TODO: consider what size to put here. make simulation handle large files
	// better with some trie-like plan? think about it...
	wal.SegmentSizeBytes = 1024 * 1024

	zapLogger := etcd.MakeZapLogger()

	// create wal
	t.Logf("creating wal")
	if wal.Exist(waldir) {
		t.Errorf("bad wal.Exists: expected false")
	}
	w, err := wal.Create(zapLogger, waldir, nil)
	if err != nil {
		t.Fatalf("wal.Create: %s", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("wal.Close: %s", err)
	}

	// open for the first time
	t.Logf("opening wal")
	w, err = wal.Open(zapLogger, waldir, walpb.Snapshot{})
	if err != nil {
		t.Fatalf("wal.Open: %s", err)
	}

	// read nothing
	t.Logf("reading wal")
	_, st, ents, err := w.ReadAll()
	if err != nil {
		t.Fatalf("wal.ReadAll: %s", err)
	}
	t.Logf("read state %v and entries %v", st, ents)
	if diff := cmp.Diff(st, raftpb.HardState{
		Term:   0,
		Vote:   0,
		Commit: 0,
	}); diff != "" {
		t.Errorf("unexpected initial state: %s", diff)
	}
	if len(ents) != 0 {
		t.Errorf("unexpected initial entries: %v", ents)
	}

	// write some data
	t.Logf("writing wal")
	if err := w.Save(raftpb.HardState{
		Term:   123,
		Vote:   456,
		Commit: 1,
	}, []raftpb.Entry{
		{
			Term:  123,
			Index: 1,
			Type:  raftpb.EntryNormal,
			Data:  []byte{1, 2, 3},
		},
		{
			Term:  123,
			Index: 2,
			Type:  raftpb.EntryNormal,
			Data:  []byte{4, 5, 6},
		},
	}); err != nil {
		t.Fatalf("wal.Save: %v", err)
	}

	// close and reopen
	t.Logf("closing and reopening wal")
	if err := w.Close(); err != nil {
		t.Fatalf("wal.Close: %s", err)
	}
	w, err = wal.Open(zapLogger, waldir, walpb.Snapshot{})
	if err != nil {
		t.Fatalf("wal.Open: %s", err)
	}

	// read data we wrote
	t.Logf("reading wal")
	_, st, ents, err = w.ReadAll()
	if err != nil {
		t.Fatalf("wal.ReadAll: %s", err)
	}
	t.Logf("read state %v and entries %v", st, ents)
	if diff := cmp.Diff(st, raftpb.HardState{
		Term:   123,
		Vote:   456,
		Commit: 1,
	}); diff != "" {
		t.Errorf("unexpected state: %s", diff)
	}
	if diff := cmp.Diff(ents, []raftpb.Entry{
		{
			Term:  123,
			Index: 1,
			Type:  raftpb.EntryNormal,
			Data:  []byte{1, 2, 3},
		},
		{
			Term:  123,
			Index: 2,
			Type:  raftpb.EntryNormal,
			Data:  []byte{4, 5, 6},
		},
	}); diff != "" {
		t.Errorf("unexpected entries: %s", diff)
	}

	// finish
	t.Logf("closing wal")
	if err := w.Close(); err != nil {
		t.Fatalf("wal.Close: %s", err)
	}
}
