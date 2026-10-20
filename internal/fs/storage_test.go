package fs

import (
	"errors"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/fs/model"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestStorageStatsAndReadOnlyLifecycle(t *testing.T) {
	s, _ := newSession(t)
	pre, err := s.Stats(ref1)
	if err != nil || pre.TotalBytes != model.DefaultCapacityBytes || pre.UsedBytes != model.StorageBlockSize || pre.FreeBytes != pre.TotalBytes-pre.UsedBytes || pre.ReadOnly {
		t.Fatalf("initial stats: %+v, %v", pre, err)
	}
	if s.machines[ref1.ID].disk != nil {
		t.Fatal("stats published a disk before start")
	}
	if err := s.ConfigureMachine(ref1, contracts.FSConfig{WorkingDir: "/app", CapacityBytes: 4 << 10, UID: 11, GID: 12}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetReadOnly(ref1, true); err != nil {
		t.Fatal(err)
	}
	i := start(t, s, ref1, 1, false)
	write(t, i.model, "/app/file", "data")
	live, err := s.Stats(ref1)
	if err != nil || live.TotalBytes != 4<<10 || live.UsedBytes != 2*model.StorageBlockSize || !live.ReadOnly {
		t.Fatalf("live stats: %+v, %v", live, err)
	}
	stop(t, i, contracts.StopGraceful)
	after, err := s.Stats(ref1)
	if err != nil || after != live {
		t.Fatalf("stopped stats: %+v, %v", after, err)
	}
	if err := s.SetReadOnly(ref1, false); err != nil {
		t.Fatal(err)
	}
	i = start(t, s, ref1, 2, true)
	if stats, err := s.Stats(ref1); err != nil || stats.ReadOnly || stats.UsedBytes != live.UsedBytes {
		t.Fatalf("restart stats: %+v, %v", stats, err)
	}
	stop(t, i, contracts.StopGraceful)
}

func TestFileCapacityAndReadOnlyWithOpenDescriptor(t *testing.T) {
	i, ctx, table := ioFixture(t)
	i.machine.config.CapacityBytes = 2 * model.StorageBlockSize
	fd := open(t, i, ctx, "limited", openCreate|2)
	resource, err := table.Lookup(fd)
	if err != nil {
		t.Fatal(err)
	}
	f := resource.(*File)
	block := make([]byte, model.StorageBlockSize)
	if n, err := f.Write(ctx, abi.ByteView{Ptr: block}); err != nil || n != len(block) {
		t.Fatalf("fill: n=%d err=%v", n, err)
	}
	before := f.pos
	if n, err := f.Write(ctx, abi.ByteView{Ptr: []byte("x")}); n != 0 || !errors.Is(err, syscall.ENOSPC) || f.pos != before {
		t.Fatalf("full write changed offset: n=%d err=%v pos=%d", n, err, f.pos)
	}
	if err := i.session.SetReadOnly(ref1, true); err != nil {
		t.Fatal(err)
	}
	if n, err := i.Pwrite(ctx, fd, abi.ByteView{Ptr: []byte("z")}, 0); n != 0 || !errors.Is(err, syscall.EROFS) {
		t.Fatalf("read-only pwrite: n=%d err=%v", n, err)
	}
	if err := i.session.SetReadOnly(ref1, false); err != nil {
		t.Fatal(err)
	}
	if n, err := i.Pwrite(ctx, fd, abi.ByteView{Ptr: []byte("z")}, 0); n != 1 || err != nil {
		t.Fatalf("writable pwrite: n=%d err=%v", n, err)
	}
}

func TestStorageConfigValidationAtomic(t *testing.T) {
	s, _ := newSession(t)
	good := contracts.FSConfig{WorkingDir: "/app", CapacityBytes: 2 << 10}
	if err := s.ConfigureMachine(ref1, good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []contracts.FSConfig{
		{WorkingDir: "/app", CapacityBytes: 1},
		{WorkingDir: "/app", LatencyNS: -1},
	} {
		if err := s.ConfigureMachine(ref1, bad); !errors.Is(err, contracts.ErrInvalidConfig) {
			t.Fatalf("accepted %+v: %v", bad, err)
		}
		if s.machines[ref1.ID].config != good {
			t.Fatal("invalid config changed machine")
		}
	}
}
