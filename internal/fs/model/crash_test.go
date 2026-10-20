package model

import (
	"os"
	"reflect"
	"slices"
	"testing"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func mustWriteModel(t *testing.T, fs *Filesystem, inode int, offset int64, data []byte) {
	t.Helper()
	n, err := fs.Write(inode, offset, abi.ByteView{Ptr: data}, DefaultCapacityBytes)
	if err != nil || n != len(data) {
		t.Fatalf("write %d bytes at %d: n=%d err=%v", len(data), offset, n, err)
	}
}

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
		ids := make([]int, 0, len(ops))
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
	for i := range count {
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
	inode, err := fs.OpenFile(RootInode, "a", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	original := make([]byte, 2*chunkSize)
	for i := range original {
		original[i] = byte(i % 251)
	}
	mustWriteModel(t, fs, inode, 0, original)
	fs.FlushEverything()
	durableChunk := fs.persisted.getFile(inode).file.chunks[0]
	// A full chunk write is retained by the pending op and the volatile state.
	changed := make([]byte, chunkSize)
	for i := range changed {
		changed[i] = 'x'
	}
	mustWriteModel(t, fs, inode, 0, changed)
	pendingChunk := fs.mem.getFile(inode).file.chunks[0]
	image := fs.FreezeCrash()
	data := image.data
	it := image.IterCrashes()
	// Flush releases the live pending write and replaces the disk. Subsequent
	// writes also exercise copy-on-write against the retained operation payload.
	fs.FlushEverything()
	mustWriteModel(t, fs, inode, 0, []byte("later"))
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
		ids := make([]int, 0, len(ops))
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
	inode, err := fs.OpenFile(RootInode, "before", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	mustWriteModel(t, fs, inode, 0, []byte("pending"))
	image := fs.FreezeCrash()
	it := image.IterCrashes()
	if err := fs.Rename(RootInode, "before", RootInode, "after", Identity{}); err != nil {
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
	if _, err := recovered.Stat(RootInode, "before", Identity{}); err != nil {
		t.Fatal("frozen directory changed", err)
	}
	if _, err := recovered.Stat(RootInode, "after", Identity{}); err == nil {
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
	inode, err := fs.OpenFile(RootInode, "mapped", os.O_CREATE|os.O_RDWR, 0o644, Identity{})
	if err != nil {
		t.Fatal(err)
	}
	mustWriteModel(t, fs, inode, 0, []byte("abcd"))
	fs.FlushEverything()
	mapping := fs.Mmap(inode, 4)
	image := fs.FreezeCrash()
	clone := image.Clone(false, nil)
	image.Release()
	defer clone.Release()
	mustWriteModel(t, fs, inode, 1, []byte("X"))
	if string(mapping.Data.Ptr) != "aXcd" {
		t.Fatal("write no longer updates mapping")
	}
	if err := fs.Truncate(inode, 2, DefaultCapacityBytes); err != nil {
		t.Fatal(err)
	}
	if string(mapping.Data.Ptr) != "aX\x00\x00" {
		t.Fatal("truncate no longer zeroes mapping")
	}
	fs.Munmap(mapping)
	fs.Release()
	mustWriteModel(t, clone, inode, 0, []byte("new!"))
	if string(mapping.Data.Ptr) != "aX\x00\x00" {
		t.Fatal("clone retained source mapping")
	}
}
