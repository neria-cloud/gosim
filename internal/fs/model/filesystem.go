package model

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"syscall"
	"unsafe"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// A Mmap tracks an open mmap for a given file. Mmaps are implemented by keeping
// a slice around with the contents of the file.
//
// Only read-only mmaps are supported since we do not get notified on writes.
// That is enough to make go.etcd.io/bbolt work.
type Mmap struct {
	Inode int
	Data  abi.ByteView
}

// next:
// - delete on test end, somehow (but pool isn't shared between tests? maybe we could/should make it?)
// - think about where we should invoke GC and why that's safe
// -- eg. when we create/apply multiple ops. should there be an API for doing that atomically?

// after a crash, iterate over all files and check for zeroes and if so, delete (optimize later: track orphans?)

// XXX: handle bad requests somehow?

type backingFile struct {
	inode int
	file  *chunkedFile
	meta  inodeMetadata

	linkCount int

	mmaps []*Mmap // open mmaps for this file
	// TODO: this is used both for the persisted and in-memory
	// file but it does not make sense for persisted files?
}

var zeroes = make([]byte, 1024)

func (f *backingFile) read(pos int64, buffer abi.ByteView) int {
	if int(pos) < f.file.size {
		n := min(buffer.Len(), f.file.size-int(pos))
		f.file.Read(int(pos), buffer.Slice(0, n))
		return n
	}
	return 0
}

func (f *backingFile) len() int {
	return f.file.size
}

func (f *backingFile) resize(size int) {
	oldSize := f.file.size
	f.file.Resize(size)

	// update mmaps
	for _, mmap := range f.mmaps {
		if size < oldSize && size <= mmap.Data.Len() {
			// Shrink and zero bytes that have been truncated.
			// When growing the file this is not necessary as
			// the default is zeroes.
			tail := mmap.Data.Slice(size, min(mmap.Data.Len(), oldSize))
			for tail.Len() > 0 {
				n := tail.Write(zeroes)
				tail = tail.SliceFrom(n)
			}
		}
	}
}

func (f *backingFile) write(pos int, size int, chunks []*refCountedChunk) {
	f.file.Write(pos, size, chunks)

	// update mmaps
	for _, mmap := range f.mmaps {
		if pos < mmap.Data.Len() {
			f.read(int64(pos), mmap.Data.Slice(pos, min(pos+size, mmap.Data.Len())))
		}
	}
}

func (f *backingFile) clone() *backingFile {
	return &backingFile{
		inode:     f.inode,
		file:      f.file.Clone(),
		meta:      f.meta,
		linkCount: f.linkCount, // XXX: test that link count is maintained
	}
}

type backingDir struct {
	inode   int
	parent  int
	name    string // name in parent
	entries map[string]int
	meta    inodeMetadata
}

type inodeMetadata struct {
	Mode uint32
	UID  uint32
	GID  uint32
}

type Identity struct{ UID, GID uint32 }

type Access uint8

const (
	AccessExecute Access = 1
	AccessWrite   Access = 2
	AccessRead    Access = 4
)

func (fs *Filesystem) CheckAccess(inode int, who Identity, need Access) error {
	// TODO(not planned): supplemental groups, ACLs and sticky-directory rules.
	meta, err := fs.metadata(inode)
	if err != nil {
		return err
	}
	if who.UID == 0 {
		return nil
	}
	bits := meta.Mode & 7
	if who.GID == meta.GID {
		bits = meta.Mode >> 3 & 7
	}
	if who.UID == meta.UID {
		bits = meta.Mode >> 6 & 7
	}
	if bits&uint32(need) != uint32(need) {
		return syscall.EACCES
	}
	return nil
}

const RootInode = 1

type filesystemState struct {
	objects map[int]any
	next    int
}

func (fss *filesystemState) clone() *filesystemState {
	objects := make(map[int]any)
	for inode, obj := range fss.objects {
		switch obj := obj.(type) {
		case *backingFile:
			objects[inode] = obj.clone()
		case *backingDir:
			objects[inode] = &backingDir{
				inode:   obj.inode,
				parent:  obj.parent,
				name:    obj.name,
				entries: maps.Clone(obj.entries),
				meta:    obj.meta,
			}
		}
	}

	return &filesystemState{
		objects: objects,
		next:    fss.next,
	}
}

// TODOs for directories:
// - handle ., .. consistently
// - garbage collect directories
// - prevent cycles
// - add deps between renames/metadata ops to prevent cycles w/ crashes

type Filesystem struct {
	// for now: flat filesystem, only one directory

	pendingOps *pendingOps

	// XXX: jank...
	openCountByInode    map[int]int
	pendingCountByInode map[int]int

	// XXX: make a different, higher-level type?
	mem       *filesystemState
	persisted *filesystemState // XXX: rename to "disk"?
}

const (
	StorageBlockSize     uint64 = chunkSize
	DefaultCapacityBytes uint64 = 128 << 20
)

type StorageStats struct {
	TotalBytes uint64
	FreeBytes  uint64
	UsedBytes  uint64
	BlockSize  uint64
}

func (fs *Filesystem) UsedBlocks() uint64 {
	var used uint64
	for inode, obj := range fs.mem.objects {
		file, ok := obj.(*backingFile)
		if !ok || file.linkCount == 0 && fs.openCountByInode[inode] == 0 {
			continue
		}
		used += file.file.AllocatedBlocks()
	}
	return used
}

func (fs *Filesystem) Stats(capacityBytes uint64) StorageStats {
	if capacityBytes == 0 {
		capacityBytes = DefaultCapacityBytes
	}
	used := fs.UsedBlocks() * StorageBlockSize
	free := uint64(0)
	if used < capacityBytes {
		free = capacityBytes - used
	}
	return StorageStats{TotalBytes: capacityBytes, FreeBytes: free, UsedBytes: used, BlockSize: StorageBlockSize}
}

func (fs *Filesystem) apply(op fsOp) {
	/*
		switch op := op.(type) {
		case *resizeOp:
			slog.Info("resize", "size", op.size)
		case *writeOp:
			slog.Info("write", "size", len(op.data), "pos", op.pos)
		}
	*/
	fs.pendingOps.addOp(op)
	op.touchedInodesIter(func(inode int) bool {
		fs.pendingCountByInode[inode]++
		return true
	})
	op.apply(fs.mem, fs)
}

func (fs *Filesystem) releaseOpRefs(ops []*pendingOp) {
	for _, op := range ops {
		op.fsOp.touchedInodesIter(func(inode int) bool {
			newCount := fs.pendingCountByInode[inode] - 1
			if newCount == 0 {
				delete(fs.pendingCountByInode, inode)
				// XXX: this check here does double work. any file whose links
				// hit zero MUST eventually be flushed by an op that refers to
				// it, which will be this op, and so here we can delete it.
				fs.maybeGC(inode)
			} else {
				fs.pendingCountByInode[inode] = newCount
			}
			return true
		})
	}
}

func (fs *Filesystem) flushInode(inode int) {
	ops := fs.pendingOps.flushInode(inode, fs.pendingOps.buffer)
	for _, op := range ops {
		op.fsOp.apply(fs.persisted, fs)
		// XXX: janky but works for now. release chunks.
		if op, ok := op.fsOp.(*writeOp); ok {
			for i, chunk := range op.chunks {
				chunk.decRef()
				op.chunks[i] = nil // set to nil just in case?
			}
			op.chunks = nil // set to nil just in case?
		}
	}

	fs.releaseOpRefs(ops)
	fs.pendingOps.releaseBuffer(ops)
}

type gcer interface {
	maybeGC(inode int)
}

type fsOp interface {
	apply(fs *filesystemState, gcer gcer)
	addDeps(p *pendingOps, op *pendingOp)
	touchedInodesIter(func(inode int) bool)
	String() string
}

func emptyState() *filesystemState {
	return &filesystemState{
		objects: map[int]any{
			RootInode: &backingDir{
				inode:   RootInode,
				parent:  RootInode,
				entries: make(map[string]int),
				meta:    inodeMetadata{Mode: 0o755},
			},
		},
		next: RootInode + 1,
	}
}

func linuxState() *filesystemState {
	// TODO: make this initialization less ugly
	return &filesystemState{
		objects: map[int]any{
			RootInode: &backingDir{
				inode:  RootInode,
				parent: RootInode,
				meta:   inodeMetadata{Mode: 0o755},
				entries: map[string]int{
					"etc": 2,
					"app": 4,
				},
			},
			2: &backingDir{
				inode:  2,
				parent: RootInode,
				name:   "etc",
				meta:   inodeMetadata{Mode: 0o755},
				entries: map[string]int{
					"hosts": 3,
				},
			},
			3: &backingFile{
				inode: 3,
				file: chunkedFileFromBytes([]byte(`# hosts
127.0.0.1 localhost
`)),
				meta:      inodeMetadata{Mode: 0o644},
				linkCount: 1,
			},
			4: &backingDir{
				inode:   4,
				parent:  RootInode,
				name:    "app",
				entries: make(map[string]int),
				meta:    inodeMetadata{Mode: 0o777},
			},
		},
		next: 5,
	}
}

func newFilesystem(initialState *filesystemState) *Filesystem {
	persisted := initialState
	mem := persisted.clone()
	return &Filesystem{
		pendingOps:          newPendingOps(),
		mem:                 mem,
		persisted:           persisted,
		openCountByInode:    make(map[int]int),
		pendingCountByInode: make(map[int]int),
	}
}

func NewEmptyFilesystem() *Filesystem {
	return newFilesystem(emptyState())
}

func NewLinuxFilesystem() *Filesystem {
	return newFilesystem(linuxState())
}

func (fs *filesystemState) getFile(idx int) *backingFile {
	return fs.objects[idx].(*backingFile)
}

func (fs *filesystemState) getFileOk(idx int) (*backingFile, bool) {
	f, ok := fs.objects[idx].(*backingFile)
	return f, ok
}

func (fs *filesystemState) getDir(idx int) (*backingDir, bool) {
	dir, ok := fs.objects[idx].(*backingDir)
	return dir, ok
}

type writeOp struct {
	// XXX: should this be an inode number?
	inode int

	pos    int
	size   int
	chunks []*refCountedChunk
}

func (o *writeOp) apply(fs *filesystemState, gcer gcer) {
	file := fs.getFile(o.inode)
	file.write(o.pos, o.size, o.chunks)
}

func (o *writeOp) touchedInodesIter(iter func(inode int) bool) {
	iter(o.inode)
}

func (o *writeOp) String() string {
	return fmt.Sprintf("write file:%d pos:%d size:%d", o.inode, o.pos, o.size)
}

type resizeOp struct {
	inode int
	size  int
}

type reserveOp struct {
	inode          int
	offset, length int
}

type metadataOp struct {
	inode int
	value inodeMetadata
}

func (o *metadataOp) apply(state *filesystemState, _ gcer) {
	switch obj := state.objects[o.inode].(type) {
	case *backingFile:
		obj.meta = o.value
	case *backingDir:
		obj.meta = o.value
	default:
		panic("metadata operation on missing inode")
	}
}

func (o *metadataOp) touchedInodesIter(iter func(inode int) bool) { iter(o.inode) }
func (o *metadataOp) String() string {
	return fmt.Sprintf("metadata inode:%d mode:%o uid:%d gid:%d", o.inode, o.value.Mode, o.value.UID, o.value.GID)
}

func (o *reserveOp) apply(state *filesystemState, _ gcer) {
	state.getFile(o.inode).file.Reserve(o.offset, o.length)
}

func (o *reserveOp) touchedInodesIter(iter func(inode int) bool) { iter(o.inode) }
func (o *reserveOp) String() string {
	return fmt.Sprintf("reserve file:%d offset:%d length:%d", o.inode, o.offset, o.length)
}

func (o *resizeOp) apply(fs *filesystemState, gcer gcer) {
	file := fs.getFile(o.inode)
	file.resize(o.size)
}

func (o *resizeOp) touchedInodesIter(iter func(inode int) bool) {
	iter(o.inode)
}

func (o *resizeOp) String() string {
	return fmt.Sprintf("resize file:%d size:%d", o.inode, o.size)
}

type dirOpPart struct {
	dir  int
	name string
	file int
}

func (p *dirOpPart) String() string {
	return fmt.Sprintf("dir:%d name:%s file:%d", p.dir, p.name, p.file)
}

type dirOp struct {
	parts []dirOpPart
}

func (o *dirOp) apply(fs *filesystemState, gcer gcer) {
	for _, p := range o.parts {
		dir, _ := fs.getDir(p.dir)

		old := dir.entries[p.name]
		if file, ok := fs.objects[old].(*backingFile); ok {
			file.linkCount--
			// slog.Info("fs link count", "inode", file.inode, "count", file.linkCount)
			if file.linkCount == 0 {
				gcer.maybeGC(old) // XXX: jank. think about locking/ordering/help
			}
		}
		if file, ok := fs.objects[old].(*backingDir); ok {
			file.parent = 0
			file.name = ""
			// GC? when?
		}

		if p.file == -1 {
			delete(dir.entries, p.name)
		} else {
			dir.entries[p.name] = p.file
			if file, ok := fs.objects[p.file].(*backingFile); ok {
				file.linkCount++
				// slog.Info("fs link count", "inode", file.inode, "count", file.linkCount)
			}
			if file, ok := fs.objects[p.file].(*backingDir); ok {
				file.parent = dir.inode
				file.name = p.name
			}
		}
	}
}

func (o *dirOp) touchedInodesIter(iter func(inode int) bool) {
	for _, p := range o.parts {
		if !iter(p.file) || !iter(p.dir) {
			return
		}
	}
}

func (o *dirOp) String() string {
	return fmt.Sprintf("dirop %v", o.parts)
}

type allocOp struct {
	dir bool

	inode int
	meta  inodeMetadata
}

func (o *allocOp) apply(fs *filesystemState, gcer gcer) {
	if _, ok := fs.objects[o.inode]; ok {
		panic("double alloc")
	}
	if o.dir {
		fs.objects[o.inode] = &backingDir{
			inode:   o.inode,
			entries: make(map[string]int),
			meta:    o.meta,
		}
	} else {
		fs.objects[o.inode] = &backingFile{
			inode: o.inode,
			file:  &chunkedFile{},
			meta:  o.meta,
		}
	}
	if o.inode >= fs.next {
		fs.next = o.inode + 1
	}
}

func (o *allocOp) touchedInodesIter(iter func(inode int) bool) {
	iter(o.inode)
}

func (o *allocOp) String() string {
	return fmt.Sprintf("alloc inode:%d dir:%v", o.inode, o.dir)
}

func (o *allocOp) addDeps(p *pendingOps, op *pendingOp) {
	p.addDep(op, depKey{kind: depKeyAlloc, inode: int32(o.inode)}, depStoreAs)
	p.addDep(op, depKey{kind: depKeyResizeFile, inode: int32(o.inode)}, depStoreAs) // optimization?
}

func (o *writeOp) addDeps(p *pendingOps, op *pendingOp) {
	p.addDep(op, depKey{kind: depKeyWriteFile, inode: int32(o.inode)}, depStoreAs)
	// p.addDep(op, depKey{kind: depKeyAlloc, inode: int32(o.inode)}, depDependsOn) // optimization?
	p.addDep(op, depKey{kind: depKeyResizeFile, inode: int32(o.inode)}, depDependsOn)
}

func (o *resizeOp) addDeps(p *pendingOps, op *pendingOp) {
	// p.addDep(op, depKey{kind: depKeyAlloc, inode: int32(o.inode)}, depDependsOn) // optimization?
	p.addDep(op, depKey{kind: depKeyResizeFile, inode: int32(o.inode)}, depStoreAsAndDependsOn)
}

func (o *reserveOp) addDeps(p *pendingOps, op *pendingOp) {
	p.addDep(op, depKey{kind: depKeyWriteFile, inode: int32(o.inode)}, depStoreAs)
	p.addDep(op, depKey{kind: depKeyResizeFile, inode: int32(o.inode)}, depDependsOn)
}

func (o *metadataOp) addDeps(p *pendingOps, op *pendingOp) {
	p.addDep(op, depKey{kind: depKeyAlloc, inode: int32(o.inode)}, depDependsOn)
	p.addDep(op, depKey{kind: depKeyMetadata, inode: int32(o.inode)}, depStoreAsAndDependsOn)
}

func (o *dirOp) addDeps(p *pendingOps, op *pendingOp) {
	for _, part := range o.parts {
		p.addDep(op, depKey{kind: depKeyAlloc, inode: int32(part.file)}, depDependsOn)
		p.addDep(op, depKey{kind: depKeyAlloc, inode: int32(part.dir)}, depDependsOn)
		p.addDep(op, depKey{kind: depKeyWriteDir, inode: int32(part.dir)}, depStoreAs)
		p.addDep(op, depKey{kind: depKeyWriteDirEntry, inode: int32(part.dir), entry: part.name}, depStoreAsAndDependsOn)
	}
}

type nilgc struct{}

func (nilgc) maybeGC(inode int) {}

func tryGetLinkCount(s *filesystemState, inode int) (int, bool) {
	switch file := s.objects[inode].(type) {
	case *backingFile:
		return file.linkCount, true
	}
	return 0, false
}

type InodeInfo struct {
	MemLinks   int
	MemExists  bool
	DiskLinks  int
	DiskExists bool
	Handles    int
	Ops        int
}

func (fs *Filesystem) getInodeInfo(inode int) InodeInfo {
	mem, memOk := tryGetLinkCount(fs.mem, inode)
	disk, diskOk := tryGetLinkCount(fs.persisted, inode)
	return InodeInfo{
		// XXX: also return if exists or not
		Handles:    fs.openCountByInode[inode],
		Ops:        fs.pendingCountByInode[inode],
		MemLinks:   mem,
		MemExists:  memOk,
		DiskLinks:  disk,
		DiskExists: diskOk,
	}
}

func (fs *Filesystem) maybeGC(inode int) {
	if inode == 2 {
		// slog.Info("considering gc", "inode", inode, "info", fs.getInodeInfo(inode))
	}

	// haha take taht!!!
	if _, ok := fs.openCountByInode[inode]; ok {
		return
	}
	if _, ok := fs.pendingCountByInode[inode]; ok {
		return
	}
	switch file := fs.mem.objects[inode].(type) {
	case *backingFile:
		if file.linkCount == 0 {
			// XXX: release the file here
			file.file.Free()
			file.file = nil
			delete(fs.mem.objects, inode)
			// delete? but what about persisted????????????????????????????????????? (must be zero also or we are in a strange situation)
			// slog.Info("fs freeing mem", "inode", inode)
		}
	}
	switch file := fs.persisted.objects[inode].(type) {
	case *backingFile:
		if file.linkCount == 0 {
			// XXX: release the file here
			file.file.Free()
			file.file = nil
			delete(fs.persisted.objects, inode)
			// delete? but what about persisted????????????????????????????????????? (must be zero also or we are in a strange situation)
			// slog.Info("fs freeing persisted", "inode", inode)
		}
	}
}

func (fs *Filesystem) Read(inode int, pos int64, buffer abi.ByteView) int {
	file := fs.mem.getFile(inode)
	return file.read(pos, buffer)
}

func (fs *Filesystem) Write(inode int, pos int64, buffer abi.ByteView, capacityBytes uint64) (int, error) {
	maxInt := int64(^uint(0) >> 1)
	if pos < 0 || pos > maxInt-int64(buffer.Len()) {
		return 0, syscall.EINVAL
	}
	file := fs.mem.getFile(inode)
	if buffer.Len() == 0 {
		return 0, nil
	}
	if capacityBytes == 0 {
		capacityBytes = DefaultCapacityBytes
	}
	limit := capacityBytes / StorageBlockSize
	needed := file.file.NewBlocksForWrite(int(pos), buffer.Len())
	if needed != 0 && (needed > limit || fs.UsedBlocks() > limit-needed) {
		return 0, syscall.ENOSPC
	}

	if int(pos)+buffer.Len() > file.len() {
		op := &resizeOp{
			inode: inode,
			size:  int(pos) + buffer.Len(),
		}
		fs.apply(op)
	}

	chunks := makeChunks(int(pos), buffer)
	len := buffer.Len()

	op := &writeOp{
		inode:  inode,
		pos:    int(pos),
		size:   buffer.Len(),
		chunks: chunks,
		// XXX: if we're running in some kind of non-fail mode, no need to make an (expensive?) copy?
		// data: data, // XXX: test that we clone this please somehow (also other clones)
	}
	fs.apply(op)

	return len, nil
}

func (fs *Filesystem) Truncate(inode int, pos int64, capacityBytes uint64) error {
	if pos < 0 || pos > int64(^uint(0)>>1) {
		return syscall.EINVAL
	}
	file := fs.mem.getFile(inode)

	if int(pos) != file.len() {
		op := &resizeOp{
			inode: inode,
			size:  int(pos),
		}
		fs.apply(op)
	}
	return nil
}

func (fs *Filesystem) Fallocate(inode int, offset, length int64, capacityBytes uint64) error {
	maxInt := int64(^uint(0) >> 1)
	if offset < 0 || length <= 0 || offset > maxInt-length {
		return syscall.EINVAL
	}
	file := fs.mem.getFile(inode)
	if capacityBytes == 0 {
		capacityBytes = DefaultCapacityBytes
	}
	limit := capacityBytes / StorageBlockSize
	needed := file.file.NewBlocksForWrite(int(offset), int(length))
	if needed != 0 && (needed > limit || fs.UsedBlocks() > limit-needed) {
		return syscall.ENOSPC
	}
	if end := int(offset + length); end > file.len() {
		fs.apply(&resizeOp{inode: inode, size: end})
	}
	if needed != 0 {
		fs.apply(&reserveOp{inode: inode, offset: int(offset), length: int(length)})
	}
	return nil
}

func (fs *Filesystem) Sync(inode int) {
	fs.flushInode(inode)
}

func (fs *Filesystem) Getdirinode(dirInode int, path string, who Identity) (int, error) {
	// For chdir
	dirInode, _, err := fs.walkpath(dirInode, path, false, who)
	if err != nil {
		return 0, err
	}

	_, ok := fs.mem.getDir(dirInode)
	if !ok {
		return 0, syscall.ENOTDIR
	}
	if err := fs.CheckAccess(dirInode, who, AccessExecute); err != nil {
		return 0, err
	}

	return dirInode, nil
}

func (fs *Filesystem) Mkdir(dirInode int, path string, mode uint32, who Identity) error {
	dirInode, name, err := fs.walkpath(dirInode, path, true, who)
	if err != nil {
		return err
	}

	dir, ok := fs.mem.getDir(dirInode)
	if !ok {
		return syscall.ENOTDIR
	}
	if err := fs.CheckAccess(dirInode, who, AccessWrite|AccessExecute); err != nil {
		return err
	}

	if _, ok := dir.entries[name]; ok {
		return syscall.EEXIST
	}

	inode := fs.mem.next
	op := &allocOp{
		dir:   true,
		inode: inode,
		meta:  inodeMetadata{Mode: mode & 0o7777 &^ 0o022, UID: who.UID, GID: who.GID},
	}
	fs.apply(op)

	dOp := &dirOp{
		parts: []dirOpPart{
			{
				dir:  dirInode,
				name: name,
				file: inode,
			},
		},
	}
	fs.apply(dOp)

	return nil
}

type StatResp struct {
	Inode int
	IsDir bool
	Size  int64
	Mode  uint32
	UID   uint32
	GID   uint32
	Nlink uint64
}

func (fs *Filesystem) statLocked(inode int) (StatResp, error) {
	obj := fs.mem.objects[inode]
	switch obj := obj.(type) {
	case *backingDir:
		return StatResp{
			Inode: inode,
			IsDir: true,
			Mode:  obj.meta.Mode,
			UID:   obj.meta.UID,
			GID:   obj.meta.GID,
			Nlink: 1,
		}, nil

	case *backingFile:
		return StatResp{
			Inode: inode,
			IsDir: false,
			Size:  int64(obj.file.size),
			Mode:  obj.meta.Mode,
			UID:   obj.meta.UID,
			GID:   obj.meta.GID,
			Nlink: uint64(obj.linkCount),
		}, nil

	default:
		panic("invalid")
	}
}

func (fs *Filesystem) Stat(dirInode int, path string, who Identity) (StatResp, error) {
	dirInode, name, err := fs.walkpath(dirInode, path, true, who)
	if err != nil {
		return StatResp{}, err
	}

	dir, ok := fs.mem.getDir(dirInode)
	if !ok {
		return StatResp{}, syscall.ENOTDIR
	}

	var inode int
	if name == "" && path == "" {
		return StatResp{}, syscall.ENOENT
	}
	if name == "." || name == "" {
		// TODO: clean up...
		inode = dirInode
	} else {
		inode, ok = dir.entries[name]
		if !ok {
			return StatResp{}, syscall.ENOENT
		}
	}
	return fs.statLocked(inode)
}

func (fs *Filesystem) Statfd(inode int) (StatResp, error) {
	return fs.statLocked(inode)
}

func (fs *Filesystem) metadata(inode int) (inodeMetadata, error) {
	switch obj := fs.mem.objects[inode].(type) {
	case *backingFile:
		return obj.meta, nil
	case *backingDir:
		return obj.meta, nil
	default:
		return inodeMetadata{}, syscall.ENOENT
	}
}

func (fs *Filesystem) Chmod(inode int, mode uint32, who Identity) error {
	meta, err := fs.metadata(inode)
	if err != nil {
		return err
	}
	if who.UID != 0 && who.UID != meta.UID {
		return syscall.EPERM
	}
	mode &= 0o7777
	if meta.Mode == mode {
		return nil
	}
	meta.Mode = mode
	fs.apply(&metadataOp{inode: inode, value: meta})
	return nil
}

func (fs *Filesystem) Chown(inode, uid, gid int, who Identity) error {
	meta, err := fs.metadata(inode)
	if err != nil {
		return err
	}
	if who.UID != 0 {
		return syscall.EPERM
	}
	if uid < -1 || gid < -1 || uint64(uid) > uint64(^uint32(0)) && uid != -1 || uint64(gid) > uint64(^uint32(0)) && gid != -1 {
		return syscall.EINVAL
	}
	next := meta
	if uid != -1 {
		next.UID = uint32(uid)
	}
	if gid != -1 {
		next.GID = uint32(gid)
	}
	if next == meta {
		return nil
	}
	fs.apply(&metadataOp{inode: inode, value: next})
	return nil
}

// for getcwd
func (fs *Filesystem) Getpath(dirInode int) (string, error) {
	var stack_ [16]string
	s := stack_[:0]
	for dirInode != RootInode {
		dir, ok := fs.mem.getDir(dirInode)
		if !ok {
			return "", syscall.ENOTDIR
		}

		s = append(s, dir.name)
		dirInode = dir.parent
	}

	if len(s) == 0 {
		return "/", nil
	}

	s = append(s, "")
	slices.Reverse(s)
	return strings.Join(s, "/"), nil
}

func (fs *Filesystem) Remove(dirInode int, path string, rmDir bool, who Identity) error {
	dirInode, name, err := fs.walkpath(dirInode, path, true, who)
	if err != nil {
		return err
	}

	dir, ok := fs.mem.getDir(dirInode)
	if !ok {
		return syscall.ENOTDIR
	}
	if err := fs.CheckAccess(dirInode, who, AccessWrite|AccessExecute); err != nil {
		return err
	}

	entry, ok := dir.entries[name]
	if !ok {
		return syscall.ENOENT
	}

	dir2, isDir := fs.mem.getDir(entry)

	// make sure isDir matches rmDir
	if !rmDir && isDir {
		return syscall.EISDIR
	}
	if rmDir && !isDir {
		return syscall.EINVAL
	}

	if isDir {
		// only allow deleting empty directories
		if len(dir2.entries) > 0 {
			return syscall.ENOTEMPTY
		}
	}

	dOp := &dirOp{
		parts: []dirOpPart{
			{
				dir:  dirInode,
				name: name,
				file: -1,
			},
		},
	}
	// XXX: clean up backing?
	fs.apply(dOp)

	return nil
}

func (fs *Filesystem) Rename(fromInode int, fromPath string, toInode int, toPath string, who Identity) error {
	fromInode, fromName, err := fs.walkpath(fromInode, fromPath, true, who)
	if err != nil {
		return err
	}

	fromDir, ok := fs.mem.getDir(fromInode)
	if !ok {
		return syscall.ENOTDIR
	}
	if err := fs.CheckAccess(fromInode, who, AccessWrite|AccessExecute); err != nil {
		return err
	}

	inode, ok := fromDir.entries[fromName]
	if !ok {
		return syscall.ENOENT
	}

	toInode, toName, err := fs.walkpath(toInode, toPath, true, who)
	if err != nil {
		return err
	}

	if _, ok := fs.mem.getDir(toInode); !ok {
		return syscall.ENOTDIR
	}
	if err := fs.CheckAccess(toInode, who, AccessWrite|AccessExecute); err != nil {
		return err
	}

	// TODO: prevent cycles?

	// slog.Info("fs rename", "from", from, "to", to, "inode", inode)

	dOp := &dirOp{
		parts: []dirOpPart{
			{
				dir:  fromInode,
				name: fromName,
				file: -1,
			},
			{
				dir:  toInode,
				name: toName,
				file: inode,
			},
		},
	}
	fs.apply(dOp)

	return nil
}

func (fs *Filesystem) OpenFile(dirInode int, path string, flag int, mode uint32, who Identity) (int, error) {
	dirInode, name, err := fs.walkpath(dirInode, path, true, who)
	if err != nil {
		return 0, err
	}

	// A trailing slash has already resolved a directory, including the root.
	// Do not look up an empty directory entry (or create one for an empty path).
	if name == "" {
		if path == "" {
			return 0, syscall.ENOENT
		}
		name = "."
	}

	dir, ok := fs.mem.getDir(dirInode)
	if !ok {
		return 0, syscall.ENOTDIR
	}

	var inode int
	created := false
	if name != "." {
		inode, ok = dir.entries[name]
		if !ok {
			if flag&os.O_CREATE == 0 {
				return 0, syscall.ENOENT
			}
			if err := fs.CheckAccess(dirInode, who, AccessWrite|AccessExecute); err != nil {
				return 0, err
			}

			inode = fs.mem.next
			op := &allocOp{
				dir:   false,
				inode: inode,
				meta:  inodeMetadata{Mode: mode & 0o7777 &^ 0o022, UID: who.UID, GID: who.GID},
			}
			fs.apply(op)

			dOp := &dirOp{
				parts: []dirOpPart{
					{
						dir:  dirInode,
						name: name,
						file: inode,
					},
				},
			}
			fs.apply(dOp)
			created = true

			// slog.Info("fs create", "path", args.Path, "inode", inode)
		}
	} else {
		// janky...
		// TODO: handle "." everywhere that reads dir, and do it cleanly
		// TODO: handle "..",
		inode = dirInode
		ok = true
	}

	if ok && flag&os.O_EXCL != 0 && flag&os.O_CREATE != 0 {
		return 0, syscall.EEXIST
	}
	if _, isDir := fs.mem.getDir(inode); !isDir {
		var need Access
		if flag&3 != os.O_WRONLY {
			need |= AccessRead
		}
		if flag&3 != os.O_RDONLY {
			need |= AccessWrite
		}
		if !created {
			if err := fs.CheckAccess(inode, who, need); err != nil {
				return 0, err
			}
		}
	} else if flag&3 != os.O_RDONLY {
		return 0, syscall.EISDIR
	}
	if ok && flag&os.O_TRUNC != 0 {
		file := fs.mem.getFile(inode)
		if file.len() != 0 {
			op := &resizeOp{
				inode: inode,
				size:  0,
			}
			fs.apply(op)
		}
	}
	fs.openCountByInode[inode]++

	return inode, nil
}

func (fs *Filesystem) CloseFile(inode int) error {
	// XXX: make this API handle based so it's less brittle?
	newCount := fs.openCountByInode[inode] - 1
	if newCount == 0 {
		delete(fs.openCountByInode, inode)
		fs.maybeGC(inode)
	} else {
		fs.openCountByInode[inode] = newCount
	}

	return nil
}

type ReadDirEntry struct {
	Name  string
	IsDir bool
}

func (fs *Filesystem) ReadDir(inode int, who Identity) ([]ReadDirEntry, error) {
	dir, ok := fs.mem.getDir(inode)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	if err := fs.CheckAccess(inode, who, AccessRead); err != nil {
		return nil, err
	}

	var entries []ReadDirEntry
	for name, inode := range dir.entries {
		var de ReadDirEntry

		obj := fs.mem.objects[inode]
		switch obj.(type) {
		case *backingDir:
			de = ReadDirEntry{
				Name:  name, // XXX?
				IsDir: true,
			}

		case *backingFile:
			de = ReadDirEntry{
				Name:  name, // XXX?
				IsDir: false,
			}

		default:
			panic("invalid")
		}

		entries = append(entries, de)
	}
	slices.SortFunc(entries, func(a, b ReadDirEntry) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return entries, nil
}

func (fs *Filesystem) GetInodeInfo(inode int) InodeInfo {
	return fs.getInodeInfo(inode)
}

func (fs *Filesystem) walkpath(baseInode int, path string, keepLast bool, who Identity) (inode int, entry string, err error) {
	// TODO(not planned): symlink traversal; the model contains only files and directories.
	// TODO: harden and test; this function and its callers are not not well
	// tested and brittle

	if len(path) > 0 && path[0:1] == "/" {
		baseInode = RootInode
	}

	inode = baseInode
	for {
		for len(path) > 0 && path[0:1] == "/" {
			path = path[1:]
		}

		nextSlash := strings.Index(path, "/")
		var name string
		if nextSlash == -1 {
			name = path
		} else {
			name = path[:nextSlash]
		}

		if len(name) == len(path) && keepLast {
			if _, ok := fs.mem.getDir(inode); !ok {
				return 0, "", syscall.ENOTDIR
			}
			if err := fs.CheckAccess(inode, who, AccessExecute); err != nil {
				return 0, "", err
			}
			return inode, name, nil
		}

		if name == "" {
			// entire path is empty? assume current dir
			if err := fs.CheckAccess(inode, who, AccessExecute); err != nil {
				return 0, "", err
			}
			return inode, "", nil
		}

		// check it is a dir
		dir, ok := fs.mem.getDir(inode)
		if !ok {
			return 0, "", syscall.ENOTDIR
		}
		if err := fs.CheckAccess(inode, who, AccessExecute); err != nil {
			return 0, "", err
		}

		if name == "." {
			// stay
			inode = dir.inode
		} else if name == ".." {
			// go up
			inode = dir.parent
		} else {
			// check it has
			entry, ok := dir.entries[name]
			if !ok {
				return 0, "", syscall.ENOENT
			}
			inode = entry
		}

		path = path[len(name):]
	}
}

// Open a mmap for the given inode. Mmap starts at offset 0.
func (fs *Filesystem) Mmap(inode int, len int) *Mmap {
	backingArray := make([]byte, len)
	byteView := abi.NewSliceView(unsafe.SliceData(backingArray), uintptr(len))

	file := fs.mem.getFile(inode)
	file.read(0, byteView)

	m := &Mmap{
		Data:  byteView,
		Inode: inode,
	}

	file.mmaps = append(file.mmaps, m)
	fs.openCountByInode[inode]++ // Mapping keeps an unlinked inode alive after fd close.
	return m
}

// Close the given mmap.
func (fs *Filesystem) Munmap(m *Mmap) {
	file := fs.mem.getFile(m.Inode)
	idx := slices.Index(file.mmaps, m)
	if idx == -1 {
		panic("not found")
	}
	file.mmaps = slices.Delete(file.mmaps, idx, idx+1)
	if err := fs.CloseFile(m.Inode); err != nil {
		panic(err)
	}
}
