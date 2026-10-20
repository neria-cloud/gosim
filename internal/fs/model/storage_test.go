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
	inode, err := fs.OpenFile(RootInode, "/app/sparse", os.O_CREATE|os.O_RDWR)
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
	if err := fs.Remove(RootInode, "/app/sparse", false); err != nil {
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
	inode, err := fs.OpenFile(RootInode, "/app/full", os.O_CREATE|os.O_RDWR)
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
	if n, err := fs.Write(inode, chunkSize, abi.ByteView{Ptr: []byte("x")}, capacity); n != 0 || !errors.Is(err, syscall.ENOSPC) {
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
	if n, err := clone.Write(inode, 0, abi.ByteView{Ptr: []byte("q")}, capacity); err != nil || n != 1 || clone.UsedBlocks() != 2 {
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
	if n, err := fs.Write(inode, 7*chunkSize, abi.ByteView{Ptr: []byte("a")}, capacity); err != nil || n != 1 || fs.UsedBlocks() != 2 {
		t.Fatalf("sparse write: n=%d err=%v used=%d", n, err, fs.UsedBlocks())
	}
	if n, err := fs.Write(inode, math.MaxInt64, abi.ByteView{Ptr: []byte("x")}, capacity); n != 0 || !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("overflow write: n=%d err=%v", n, err)
	}
	if n, err := fs.Write(inode, 9*chunkSize, abi.ByteView{}, capacity); err != nil || n != 0 {
		t.Fatalf("empty write: n=%d err=%v", n, err)
	}
}
