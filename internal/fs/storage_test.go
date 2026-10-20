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

func TestFallocateProviderReservesAndRejectsFullVolume(t *testing.T) {
	i, ctx, _ := ioFixture(t)
	i.machine.config.CapacityBytes = 3 * model.StorageBlockSize
	fd := open(t, i, ctx, "allocated", openCreate|2)
	if err := i.Fallocate(ctx, fd, 0, 512, 1024); err != nil {
		t.Fatal(err)
	}
	stats, err := i.session.Stats(ref1)
	if err != nil || stats.UsedBytes != 3*model.StorageBlockSize {
		t.Fatalf("reserved stats: %+v, %v", stats, err)
	}
	if err := i.Fallocate(ctx, fd, 0, 2048, 1); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("full fallocate: %v", err)
	}
	if stat, err := i.Fstat(ctx, fd); err != nil || stat.Size != 1536 {
		t.Fatalf("failed fallocate changed size: %+v, %v", stat, err)
	}
	if err := i.Sync(ctx, fd, false); err != nil {
		t.Fatal(err)
	}
}

func TestStatReportsStoredModeAndOwnership(t *testing.T) {
	i, ctx, table := ioFixture(t)
	fd := open(t, i, ctx, "owned", openCreate|2)
	resource, err := table.Lookup(fd)
	if err != nil {
		t.Fatal(err)
	}
	inode := resource.(*File).inode
	if err := i.model.Chmod(inode, 0o640, model.Identity{}); err != nil {
		t.Fatal(err)
	}
	if err := i.model.Chown(inode, 101, 202, model.Identity{}); err != nil {
		t.Fatal(err)
	}
	for _, stat := range []func() (contracts.Stat, error){
		func() (contracts.Stat, error) { return i.Fstat(ctx, fd) },
		func() (contracts.Stat, error) { return i.Statat(ctx, atFDCWD, "owned", 0) },
	} {
		got, err := stat()
		if err != nil || got.Mode != 0o100640 || got.UID != 101 || got.GID != 202 || got.Nlink != 1 {
			t.Fatalf("stat metadata: %+v, %v", got, err)
		}
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

func TestIdentityAndReadOnlyMutations(t *testing.T) {
	i, ctx, _ := ioFixture(t)
	i.machine.config.UID, i.machine.config.GID = 101, 202
	fd := open(t, i, ctx, "mine", openCreate|2)
	stat, err := i.Fstat(ctx, fd)
	if err != nil || stat.Mode != 0o100600 || stat.UID != 101 || stat.GID != 202 {
		t.Fatalf("created metadata: %+v, %v", stat, err)
	}
	if _, err := i.Openat(ctx, atFDCWD, "/etc/hosts", 2, 0); !errors.Is(err, syscall.EACCES) {
		t.Fatalf("non-root write to hosts: %v", err)
	}
	if err := i.Mkdirat(ctx, atFDCWD, "sub", 0o777); err != nil {
		t.Fatal(err)
	}
	dir, err := i.Statat(ctx, atFDCWD, "sub", 0)
	if err != nil || dir.Mode != 0o040755 || dir.UID != 101 || dir.GID != 202 {
		t.Fatalf("created directory: %+v, %v", dir, err)
	}
	if err := i.session.SetReadOnly(ref1, true); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func() error{
		"mkdir":    func() error { return i.Mkdirat(ctx, atFDCWD, "blocked", 0o777) },
		"unlink":   func() error { return i.Unlinkat(ctx, atFDCWD, "mine", 0) },
		"rename":   func() error { return i.Renameat(ctx, atFDCWD, "mine", atFDCWD, "renamed") },
		"truncate": func() error { return i.Truncate(ctx, fd, 0) },
		"reserve":  func() error { return i.Fallocate(ctx, fd, 0, 0, 1) },
	} {
		if err := mutate(); !errors.Is(err, syscall.EROFS) {
			t.Fatalf("read-only %s: %v", name, err)
		}
	}
	if _, err := i.Openat(ctx, atFDCWD, "mine", openTruncate|2, 0); !errors.Is(err, syscall.EROFS) {
		t.Fatalf("read-only truncate open: %v", err)
	}
	if err := i.session.SetReadOnly(ref1, false); err != nil {
		t.Fatal(err)
	}
	if err := i.Renameat(ctx, atFDCWD, "mine", atFDCWD, "renamed"); err != nil {
		t.Fatal(err)
	}
	if err := i.Unlinkat(ctx, atFDCWD, "renamed", 0); err != nil {
		t.Fatal(err)
	}
}
