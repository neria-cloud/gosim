package model

import (
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// This test oracle keeps the former recursive traversal over live dependency
// nodes, independent of the snapshot representation and explicit stack.
func compareCrashOrder(t *testing.T, pending *pendingOps) {
	t.Helper()
	ops := pending.allOps()
	unavailable := make(map[depKey]int)
	var picked []int
	var want [][]int
	var search func(int)
	search = func(index int) {
		if index == len(ops) {
			want = append(want, slices.Clone(picked))
			return
		}
		op := ops[index]
		valid := true
		for dep := op.first; dep != nil; dep = dep.sibling {
			if dep.kind.isDependsOn() && unavailable[dep.key] > 0 {
				valid = false
			}
		}
		if valid {
			picked = append(picked, op.fsOp.(*allocOp).inode)
			search(index + 1)
			picked = picked[:len(picked)-1]
		}
		for dep := op.first; dep != nil; dep = dep.sibling {
			if dep.kind.isStoreAs() {
				unavailable[dep.key]++
			}
		}
		search(index + 1)
		for dep := op.first; dep != nil; dep = dep.sibling {
			if dep.kind.isStoreAs() {
				unavailable[dep.key]--
			}
		}
	}
	search(0)
	plan := pending.snapshot()
	defer plan.release()
	it := plan.iterator()
	defer it.close()
	var got [][]int
	for {
		ops, ok := it.next()
		if !ok {
			break
		}
		var ids []int
		for _, op := range ops {
			ids = append(ids, op.(*allocOp).inode)
		}
		got = append(got, ids)
	}
	if !slices.EqualFunc(want, got, slices.Equal[[]int]) {
		t.Fatalf("crash order: got %v, want %v", got, want)
	}
}

func TestCrashPlanDeepAndCopiedResults(t *testing.T) {
	t.Parallel()
	const count = 100000
	p := &crashPlan{}
	for i := 0; i < count; i++ {
		op := plannedOp{op: &allocOp{inode: i}, stores: []depKey{{inode: int32(i + 1)}}}
		if i > 0 {
			op.depends = []depKey{{inode: int32(i)}}
		}
		p.ops = append(p.ops, op)
	}
	it := p.iterator()
	first, ok := it.next()
	if !ok || len(first) != count {
		t.Fatal("missing full chain")
	}
	second, ok := it.next()
	if !ok || len(second) != count-1 {
		t.Fatal("missing next chain prefix")
	}
	if first[count-1] == nil {
		t.Fatal("returned operation slice aliases traversal")
	}
	it.close()
	it.close()
	if _, ok := it.next(); ok {
		t.Fatal("closed iterator yielded")
	}
	if it.plan != nil || it.stack != nil || it.picked != nil || it.unavailable != nil {
		t.Fatal("closed iterator retains plan")
	}
}

func TestCrashDependencyCountOverflow(t *testing.T) {
	t.Parallel()
	key := depKey{inode: 1}
	defer func() {
		if recover() == nil {
			t.Fatal("overflow accepted")
		}
	}()
	incrementUnavailable([]depKey{key}, map[depKey]uint32{key: ^uint32(0)})
}

func TestCrashImageEmptyPlan(t *testing.T) {
	t.Parallel()
	fs := NewEmptyFilesystem()
	image := fs.FreezeCrash()
	it := image.IterCrashes()
	fs.Release()
	image.Release()
	state, ok := it.Next()
	if !ok {
		t.Fatal("empty plan has no durable image")
	}
	if _, ok := it.Next(); ok {
		t.Fatal("empty plan has multiple images")
	}
	if it.data != nil {
		t.Fatal("exhausted iterator retains image")
	}
	state.Release()
	it.Close()
}

func TestCrashImageRetention(t *testing.T) {
	t.Parallel()
	fs := NewEmptyFilesystem()
	inode, err := fs.OpenFile(RootInode, "a", os.O_CREATE|os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	original := make([]byte, 2*chunkSize)
	for i := range original {
		original[i] = byte(i % 251)
	}
	fs.Write(inode, 0, abi.ByteView{Ptr: original})
	fs.FlushEverything()
	durableChunk := fs.persisted.getFile(inode).file.chunks[0]
	// A full chunk write is retained by the pending op and the volatile state.
	changed := make([]byte, chunkSize)
	for i := range changed {
		changed[i] = 'x'
	}
	fs.Write(inode, 0, abi.ByteView{Ptr: changed})
	pendingChunk := fs.mem.getFile(inode).file.chunks[0]
	image := fs.FreezeCrash()
	data := image.data
	it := image.IterCrashes()
	// Flush releases the live pending write and replaces the disk. Subsequent
	// writes also exercise copy-on-write against the retained operation payload.
	fs.FlushEverything()
	fs.Write(inode, 0, abi.ByteView{Ptr: []byte("later")})
	fs.Release()
	fs.Release()
	disk := image.Clone(false, nil)
	image.Release()
	image.Release()
	recovered, ok := it.Next()
	if !ok {
		t.Fatal("iterator lost retained image")
	}
	it.Close()
	it.Close()
	if data.refs != 0 || data.disk != nil || data.plan != nil {
		t.Fatal("snapshot retained after last owner closed")
	}
	check := func(fs *Filesystem, want []byte) {
		t.Helper()
		got := make([]byte, len(want))
		if n := fs.Read(inode, 0, abi.ByteView{Ptr: got}); n != len(got) || !slices.Equal(got, want) {
			t.Fatal("snapshot content changed")
		}
	}
	check(disk, original)
	check(recovered, append(slices.Clone(changed), original[chunkSize:]...))
	disk.Release()
	recovered.Release()
	if durableChunk.refs != 0 || pendingChunk.refs != 0 || durableChunk.data != nil || pendingChunk.data != nil {
		t.Fatal("retained chunks leaked")
	}
}

func TestCrashSnapshotDependencyMetadata(t *testing.T) {
	t.Parallel()
	p := newPendingOps()
	first := p.addOpInternal()
	first.fsOp = &allocOp{inode: 1}
	p.addDep(first, depKey{inode: 1}, depStoreAs)
	second := p.addOpInternal()
	second.fsOp = &allocOp{inode: 2}
	p.addDep(second, depKey{inode: 1}, depDependsOn)
	p.addDep(second, depKey{inode: 2}, depStoreAs)
	plan := p.snapshot()
	defer plan.release()
	first.fsOp.(*allocOp).inode = 99
	first.first.key.inode = 99
	p.release()
	it := plan.iterator()
	defer it.close()
	var got [][]int
	for {
		ops, ok := it.next()
		if !ok {
			break
		}
		var ids []int
		for _, op := range ops {
			ids = append(ids, op.(*allocOp).inode)
		}
		got = append(got, ids)
	}
	if !reflect.DeepEqual(got, [][]int{{1, 2}, {1}, nil}) {
		t.Fatalf("mutable snapshot: %v", got)
	}
}

func TestCrashRandomIndependentAndDependencyValid(t *testing.T) {
	t.Parallel()
	p := &crashPlan{ops: []plannedOp{
		{op: &allocOp{inode: 1}, stores: []depKey{{inode: 1}}},
		{op: &allocOp{inode: 2}, depends: []depKey{{inode: 1}}},
	}}
	a, b := gosimruntime.NewDomainRandom(42, "fs", 7), gosimruntime.NewDomainRandom(42, "fs", 7)
	app, net := gosimruntime.NewDomainRandom(42, "application", 7), gosimruntime.NewDomainRandom(42, "net", 7)
	selected := 0
	for i := 0; i < 3000; i++ {
		app.Uint64()
		net.Uint64()
		got, want := p.randomCrashSubset(a), p.randomCrashSubset(b)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("other domain changed filesystem selection")
		}
		if len(got) > 0 {
			selected++
			if got[0].(*allocOp).inode != 1 {
				t.Fatal("selected dependent without prerequisite")
			}
		}
	}
	if selected < 1800 || selected > 2200 {
		t.Fatalf("unexpected pick rate: %d/3000", selected)
	}
}

func TestChunkCopyOnWriteReleasesOldReference(t *testing.T) {
	t.Parallel()
	a := chunkedFileFromBytes([]byte("original"))
	chunk := a.chunks[0]
	b := a.Clone()
	b.ensureWritableChunk(0).data[0] = 'X'
	if chunk.refs != 1 || a.chunks[0].data[0] != 'o' {
		t.Fatal("copy-on-write ownership failure")
	}
	a.Free()
	a.Free()
	b.Free()
	if chunk.refs != 0 {
		t.Fatal("copy-on-write leaked old reference")
	}
}

func TestCrashImageOwnsDirectoryOperations(t *testing.T) {
	t.Parallel()
	fs := NewEmptyFilesystem()
	inode, err := fs.OpenFile(RootInode, "before", os.O_CREATE|os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	fs.Write(inode, 0, abi.ByteView{Ptr: []byte("pending")})
	image := fs.FreezeCrash()
	it := image.IterCrashes()
	if err := fs.Rename(RootInode, "before", RootInode, "after"); err != nil {
		t.Fatal(err)
	}
	fs.FlushEverything()
	fs.Release()
	image.Release()
	recovered, ok := it.Next()
	if !ok {
		t.Fatal("missing recovery")
	}
	it.Close()
	defer recovered.Release()
	if _, err := recovered.Stat(RootInode, "before"); err != nil {
		t.Fatal("frozen directory changed", err)
	}
	if _, err := recovered.Stat(RootInode, "after"); err == nil {
		t.Fatal("later rename entered snapshot")
	}
	got := make([]byte, 7)
	recovered.Read(inode, 0, abi.ByteView{Ptr: got})
	if string(got) != "pending" {
		t.Fatalf("pending payload not retained: %q", got)
	}
}

func TestCrashCloneDoesNotRetainMappings(t *testing.T) {
	t.Parallel()
	fs := NewEmptyFilesystem()
	inode, err := fs.OpenFile(RootInode, "mapped", os.O_CREATE|os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	fs.Write(inode, 0, abi.ByteView{Ptr: []byte("abcd")})
	fs.FlushEverything()
	mapping := fs.Mmap(inode, 4)
	image := fs.FreezeCrash()
	clone := image.Clone(false, nil)
	image.Release()
	defer clone.Release()
	fs.Write(inode, 1, abi.ByteView{Ptr: []byte("X")})
	if string(mapping.Data.Ptr) != "aXcd" {
		t.Fatal("write no longer updates mapping")
	}
	fs.Truncate(inode, 2)
	if string(mapping.Data.Ptr) != "aX\x00\x00" {
		t.Fatal("truncate no longer zeroes mapping")
	}
	fs.Munmap(mapping)
	fs.Release()
	clone.Write(inode, 0, abi.ByteView{Ptr: []byte("new!")})
	if string(mapping.Data.Ptr) != "aX\x00\x00" {
		t.Fatal("clone retained source mapping")
	}
}
