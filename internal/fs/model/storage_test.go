package model

import (
	"os"
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
	fs.Truncate(inode, 4096)
	if got := fs.UsedBlocks(); got != 1 {
		t.Fatalf("sparse grow allocated %d blocks", got)
	}
	fs.Write(inode, 2048, abi.ByteView{Ptr: []byte("x")})
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
