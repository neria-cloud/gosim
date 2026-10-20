package model

import (
	"slices"

	"github.com/neria-cloud/gosim/internal/contracts"
)

// CrashImage retains an immutable durable state and pending write plan. All
// model operations are owned by the supervisor; reference counts need no locks.
type (
	CrashImage struct{ data *crashData }
	crashData  struct {
		disk *filesystemState
		plan *crashPlan
		refs int
	}
)

func (fs *Filesystem) FreezeCrash() *CrashImage {
	return &CrashImage{data: &crashData{disk: fs.persisted.clone(), plan: fs.pendingOps.snapshot(), refs: 1}}
}

func (d *crashData) retain() {
	if d.refs <= 0 || d.refs == int(^uint(0)>>1) {
		panic("invalid crash image reference count")
	}
	d.refs++
}

func (d *crashData) release() {
	if d.refs <= 0 {
		panic("released crash image")
	}
	d.refs--
	if d.refs == 0 {
		d.disk.release()
		d.plan.release()
		d.disk, d.plan = nil, nil
	}
}

func (image *CrashImage) Release() {
	if image.data != nil {
		image.data.release()
		image.data = nil
	}
}

func (d *crashData) clone(ops []fsOp) *Filesystem {
	disk := d.disk.clone()
	for _, op := range ops {
		op.apply(disk, nilgc{})
	}
	// Preserve existing crash semantics, including unreachable inode retention.
	return newFilesystem(disk)
}

func (image *CrashImage) Clone(partial bool, random contracts.Random) *Filesystem {
	if image.data == nil {
		panic("released crash image")
	}
	var ops []fsOp
	if partial {
		ops = image.data.plan.randomCrashSubset(random)
	}
	return image.data.clone(ops)
}

type CrashIterator struct {
	data *crashData
	plan *crashPlanIterator
}

func (image *CrashImage) IterCrashes() *CrashIterator {
	if image.data == nil {
		panic("released crash image")
	}
	image.data.retain()
	return &CrashIterator{data: image.data, plan: image.data.plan.iterator()}
}

func (it *CrashIterator) Next() (*Filesystem, bool) {
	if it.data == nil {
		return nil, false
	}
	ops, ok := it.plan.next()
	if !ok {
		it.Close()
		return nil, false
	}
	return it.data.clone(ops), true
}

func (it *CrashIterator) Close() {
	if it.data != nil {
		it.plan.close()
		it.data.release()
		it.data, it.plan = nil, nil
	}
}

func (state *filesystemState) release() {
	if state == nil {
		return
	}
	for _, object := range state.objects {
		if file, ok := object.(*backingFile); ok {
			file.file.Free()
			file.mmaps = nil
		}
	}
	state.objects = nil
}

func releaseOp(op fsOp) {
	if write, ok := op.(*writeOp); ok {
		for _, chunk := range write.chunks {
			chunk.decRef()
		}
		write.chunks = nil
	}
}

func retainOp(op fsOp) fsOp {
	switch typedOp := op.(type) {
	case *writeOp:
		writeCopy := *typedOp
		writeCopy.chunks = slices.Clone(typedOp.chunks)
		for _, chunk := range writeCopy.chunks {
			chunk.incRef()
		}
		return &writeCopy
	case *resizeOp:
		resizeCopy := *typedOp
		return &resizeCopy
	case *reserveOp:
		reserveCopy := *typedOp
		return &reserveCopy
	case *metadataOp:
		metadataCopy := *typedOp
		return &metadataCopy
	case *allocOp:
		allocCopy := *typedOp
		return &allocCopy
	case *dirOp:
		dirCopy := *typedOp
		dirCopy.parts = slices.Clone(typedOp.parts)
		return &dirCopy
	default:
		panic("unknown filesystem operation")
	}
}

func (p *pendingOps) release() {
	for _, op := range p.allOps() {
		releaseOp(op.fsOp)
	}
	p.lastDepForKey, p.buffer = nil, nil
}

// Release drops this instance's block, pending-data, and mapping references.
// Snapshots, iterators, and clones retain their own references.
func (fs *Filesystem) Release() {
	if fs.mem == nil {
		return
	}
	fs.mem.release()
	fs.persisted.release()
	fs.pendingOps.release()
	fs.mem, fs.persisted, fs.pendingOps = nil, nil, nil
	fs.openCountByInode, fs.pendingCountByInode = nil, nil
}

func (fs *Filesystem) FlushEverything() {
	disk := fs.mem.clone()
	fs.persisted.release()
	fs.pendingOps.release()
	fs.persisted = disk
	fs.pendingOps = newPendingOps()
	fs.pendingCountByInode = make(map[int]int)
}
