package model

import (
	"errors"
	"math"
	"os"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestUsedBlocksCountsAllocatedLiveFiles(t *testing.T) {
	fs := NewLinuxFilesystem()
	defer fs.Release()
	if got := fs.UsedBlocks(); got != 1 {
		t.Fatalf("initial blocks = %d, want hosts block", got)
	}
	inode, err := fs.OpenFile(RootInode, "/app/sparse", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.Truncate(inode, 4096, DefaultCapacityBytes); err != nil {
		t.Fatal(err)
	}
	if got := fs.UsedBlocks(); got != 1 {
		t.Fatalf("sparse grow allocated %d blocks", got)
	}
	if _, err := fs.Write(inode, 2048, abi.ByteView{Ptr: []byte("x")}, DefaultCapacityBytes); err != nil {
		t.Fatal(err)
	}
	if got := fs.UsedBlocks(); got != 2 {
		t.Fatalf("write allocated %d blocks", got)
	}
	if err := fs.Remove(RootInode, "/app/sparse", false, Identity{}); err != nil {
		t.Fatal(err)
	}
	if got := fs.UsedBlocks(); got != 2 {
		t.Fatalf("unlinked open file lost charge: %d", got)
	}
	if err := fs.CloseFile(inode); err != nil {
		t.Fatal(err)
	}
	if got := fs.UsedBlocks(); got != 1 {
		t.Fatalf("closed unlinked file remains charged: %d", got)
	}
}

func TestCapacityAdmissionIsAtomic(t *testing.T) {
	fs := NewLinuxFilesystem()
	defer fs.Release()
	inode, err := fs.OpenFile(RootInode, "/app/full", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	defer fs.CloseFile(inode)
	const capacity = 2 * StorageBlockSize // one block is /etc/hosts
	full := make([]byte, chunkSize)
	if n, err := fs.Write(inode, 0, abi.ByteView{Ptr: full}, capacity); err != nil || n != len(full) {
		t.Fatalf("fill: n=%d err=%v", n, err)
	}
	before := fs.GetInodeInfo(inode).Ops
	if n, err := fs.Write(inode, chunkSize, abi.ByteView{Ptr: []byte("x")}, capacity); n != 0 ||
		!errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("overcapacity write: n=%d err=%v", n, err)
	}
	if stat, err := fs.Statfd(inode); err != nil || stat.Size != chunkSize || fs.GetInodeInfo(inode).Ops != before {
		t.Fatalf("failed write changed state: stat=%+v err=%v", stat, err)
	}
	if n, err := fs.Write(inode, chunkSize-1, abi.ByteView{Ptr: []byte("z")}, capacity); err != nil || n != 1 {
		t.Fatalf("overwrite: n=%d err=%v", n, err)
	}
	fs.FlushEverything()
	image := fs.FreezeCrash()
	clone := image.Clone(false, nil)
	image.Release()
	if n, err := clone.Write(inode, 0, abi.ByteView{Ptr: []byte("q")}, capacity); err != nil || n != 1 ||
		clone.UsedBlocks() != 2 {
		t.Fatalf("copy-on-write clone consumed virtual capacity: n=%d err=%v blocks=%d", n, err, clone.UsedBlocks())
	}
	clone.Release()
	if err := fs.Truncate(inode, 0, capacity); err != nil {
		t.Fatal(err)
	}
	if stats := fs.Stats(capacity); stats.UsedBytes != StorageBlockSize {
		t.Fatalf("truncate did not free block: %+v", stats)
	}
	if err := fs.Truncate(inode, 8*chunkSize, capacity); err != nil || fs.UsedBlocks() != 1 {
		t.Fatalf("sparse growth allocated blocks: %v, %d", err, fs.UsedBlocks())
	}
	if n, err := fs.Write(inode, 7*chunkSize, abi.ByteView{Ptr: []byte("a")}, capacity); err != nil || n != 1 ||
		fs.UsedBlocks() != 2 {
		t.Fatalf("sparse write: n=%d err=%v used=%d", n, err, fs.UsedBlocks())
	}
	if n, err := fs.Write(inode, math.MaxInt64, abi.ByteView{Ptr: []byte("x")}, capacity); n != 0 ||
		!errors.Is(err, syscall.EINVAL) {
		t.Fatalf("overflow write: n=%d err=%v", n, err)
	}
	if n, err := fs.Write(inode, 9*chunkSize, abi.ByteView{}, capacity); err != nil || n != 0 {
		t.Fatalf("empty write: n=%d err=%v", n, err)
	}
}

func TestOverCapacityImageAllowsExistingBlocks(t *testing.T) {
	fs := NewLinuxFilesystem()
	defer fs.Release()
	inode, err := fs.OpenFile(RootInode, "/app/recovered", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	defer fs.CloseFile(inode)
	if _, err := fs.Write(inode, 0, abi.ByteView{Ptr: make([]byte, 2*chunkSize)}, DefaultCapacityBytes); err != nil {
		t.Fatal(err)
	}
	const capacity = 2 * StorageBlockSize
	if stats := fs.Stats(capacity); stats.UsedBytes != 3*StorageBlockSize || stats.FreeBytes != 0 {
		t.Fatalf("over-capacity stats: %+v", stats)
	}
	if n, err := fs.Write(inode, 0, abi.ByteView{Ptr: []byte("x")}, capacity); n != 1 || err != nil {
		t.Fatalf("existing-block overwrite: n=%d err=%v", n, err)
	}
	if err := fs.Fallocate(inode, 0, 2*chunkSize, capacity); err != nil {
		t.Fatalf("existing-block reservation: %v", err)
	}
	if n, err := fs.Write(inode, 2*chunkSize, abi.ByteView{Ptr: []byte("x")}, capacity); n != 0 ||
		!errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("new allocation above capacity: n=%d err=%v", n, err)
	}
	if err := fs.Truncate(inode, 0, capacity); err != nil {
		t.Fatal(err)
	}
	if n, err := fs.Write(inode, 0, abi.ByteView{Ptr: []byte("x")}, capacity); n != 1 || err != nil {
		t.Fatalf("new allocation after shrink: n=%d err=%v", n, err)
	}
}

func TestFallocateReservesBlocksAndCrashState(t *testing.T) {
	fs := NewLinuxFilesystem()
	defer fs.Release()
	inode, err := fs.OpenFile(RootInode, "/app/reserved", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	defer fs.CloseFile(inode)
	fs.FlushEverything()
	const capacity = 3 * StorageBlockSize // hosts plus two reserved blocks
	if err := fs.Fallocate(inode, 512, 1024, capacity); err != nil {
		t.Fatal(err)
	}
	if stat, err := fs.Statfd(inode); err != nil || stat.Size != 1536 || fs.UsedBlocks() != 3 {
		t.Fatalf("reservation did not allocate range: stat=%+v err=%v blocks=%d", stat, err, fs.UsedBlocks())
	}
	zeros := make([]byte, 1024)
	if n := fs.Read(inode, 512, abi.ByteView{Ptr: zeros}); n != len(zeros) {
		t.Fatalf("reserved range read %d bytes", n)
	}
	for _, b := range zeros {
		if b != 0 {
			t.Fatal("reserved range is not zero-filled")
		}
	}
	before := fs.GetInodeInfo(inode).Ops
	if err := fs.Fallocate(inode, 0, 1024, capacity); err != nil || fs.UsedBlocks() != 3 {
		t.Fatalf("overlapping reserve: err=%v blocks=%d", err, fs.UsedBlocks())
	}
	if err := fs.Fallocate(inode, 2048, 1, capacity); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("overcapacity reserve: %v", err)
	}
	if stat, _ := fs.Statfd(inode); stat.Size != 1536 || fs.GetInodeInfo(inode).Ops != before {
		t.Fatal("failed reservation changed file or pending operations")
	}
	image := fs.FreezeCrash()
	iterator := image.IterCrashes()
	seenPending, seenDurable := false, false
	for {
		state, ok := iterator.Next()
		if !ok {
			break
		}
		switch state.UsedBlocks() {
		case 1:
			seenDurable = true
		case 3:
			seenPending = true
		default:
			t.Fatalf("unexpected crash usage %d", state.UsedBlocks())
		}
		state.Release()
	}
	image.Release()
	if !seenPending || !seenDurable {
		t.Fatalf("crash outcomes omitted reservation: durable=%v pending=%v", seenDurable, seenPending)
	}
	fs.Sync(inode)
	image = fs.FreezeCrash()
	recovered := image.Clone(false, nil)
	image.Release()
	if recovered.UsedBlocks() != 3 {
		t.Fatalf("sync lost reservation: %d", recovered.UsedBlocks())
	}
	recovered.Release()
	if err := fs.Truncate(inode, 0, capacity); err != nil || fs.UsedBlocks() != 1 {
		t.Fatalf("truncate did not free reservation: %v, %d", err, fs.UsedBlocks())
	}
}

func TestMetadataOwnershipAndDurability(t *testing.T) {
	fs := NewLinuxFilesystem()
	defer fs.Release()
	inode, err := fs.OpenFile(RootInode, "/app/meta", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	defer fs.CloseFile(inode)
	fs.FlushEverything()
	initial, err := fs.Statfd(inode)
	if err != nil || initial.Mode != 0o644 || initial.UID != 0 || initial.GID != 0 || initial.Nlink != 1 {
		t.Fatalf("initial metadata: %+v, %v", initial, err)
	}
	if err := fs.Chmod(inode, 0o600, Identity{UID: 7}); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("unowned chmod: %v", err)
	}
	if err := fs.Chmod(inode, 0o600, Identity{}); err != nil {
		t.Fatal(err)
	}
	if err := fs.Chown(inode, 27, 31, Identity{}); err != nil {
		t.Fatal(err)
	}
	if err := fs.Chown(inode, -1, 32, Identity{}); err != nil {
		t.Fatal(err)
	}
	if err := fs.Chmod(inode, 0o640, Identity{UID: 27}); err != nil {
		t.Fatal(err)
	}
	if err := fs.Chown(inode, 1, 1, Identity{UID: 27}); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("non-root chown: %v", err)
	}
	before := fs.GetInodeInfo(inode).Ops
	if err := fs.Chmod(inode, 0o640, Identity{UID: 27}); err != nil || fs.GetInodeInfo(inode).Ops != before {
		t.Fatal("identical chmod created pending op", err)
	}
	current, err := fs.Statfd(inode)
	if err != nil || current.Mode != 0o640 || current.UID != 27 || current.GID != 32 {
		t.Fatalf("live metadata: %+v, %v", current, err)
	}
	image := fs.FreezeCrash()
	iter := image.IterCrashes()
	seenOld, seenNew := false, false
	for {
		state, ok := iter.Next()
		if !ok {
			break
		}
		stat, err := state.Statfd(inode)
		if err != nil {
			t.Fatal(err)
		}
		seenOld = seenOld || stat.Mode == 0o644 && stat.UID == 0
		seenNew = seenNew || stat.Mode == 0o640 && stat.UID == 27 && stat.GID == 32
		state.Release()
	}
	image.Release()
	if !seenOld || !seenNew {
		t.Fatalf("metadata crash outcomes old=%v new=%v", seenOld, seenNew)
	}
	fs.Sync(inode)
	image = fs.FreezeCrash()
	recovered := image.Clone(false, nil)
	image.Release()
	stat, err := recovered.Statfd(inode)
	if err != nil || stat.Mode != 0o640 || stat.UID != 27 || stat.GID != 32 {
		t.Fatalf("synced metadata: %+v, %v", stat, err)
	}
	recovered.Release()
}

func TestIdentityAccessAndCreationModes(t *testing.T) {
	fs := NewLinuxFilesystem()
	defer fs.Release()
	user := Identity{UID: 123, GID: 456}
	if err := fs.Mkdir(RootInode, "/app/private", 0o777, user); err != nil {
		t.Fatal(err)
	}
	dir, err := fs.Getdirinode(RootInode, "/app/private", user)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := fs.Stat(RootInode, "/app/private", user)
	if err != nil || stat.Mode != 0o755 || stat.UID != user.UID || stat.GID != user.GID {
		t.Fatalf("directory metadata: %+v, %v", stat, err)
	}
	inode, err := fs.OpenFile(dir, "secret", os.O_CREATE|os.O_RDWR, 0o666, user)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.CloseFile(inode)
	stat, err = fs.Statfd(inode)
	if err != nil || stat.Mode != 0o644 || stat.UID != user.UID || stat.GID != user.GID {
		t.Fatalf("file metadata: %+v, %v", stat, err)
	}
	other := Identity{UID: 789, GID: 999}
	if err := fs.Chmod(inode, 0o600, user); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.OpenFile(dir, "secret", os.O_RDONLY, 0, other); !errors.Is(err, syscall.EACCES) {
		t.Fatalf("other user read: %v", err)
	}
	if _, err := fs.OpenFile(dir, "secret", os.O_WRONLY, 0, other); !errors.Is(err, syscall.EACCES) {
		t.Fatalf("other user write: %v", err)
	}
	if err := fs.Chmod(dir, 0o700, user); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(RootInode, "/app/private/secret", other); !errors.Is(err, syscall.EACCES) {
		t.Fatalf("blocked traversal: %v", err)
	}
	if err := fs.Remove(RootInode, "/app/private/secret", false, other); !errors.Is(err, syscall.EACCES) {
		t.Fatalf("blocked removal: %v", err)
	}
	if _, err := fs.ReadDir(dir, other); !errors.Is(err, syscall.EACCES) {
		t.Fatalf("blocked directory listing: %v", err)
	}
	if err := fs.Chmod(inode, 0o644, other); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("non-owner chmod: %v", err)
	}
	if err := fs.Chown(inode, 1, -1, user); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("non-root chown: %v", err)
	}
	if err := fs.Remove(RootInode, "/app/private/secret", false, user); err != nil {
		t.Fatal(err)
	}
}
