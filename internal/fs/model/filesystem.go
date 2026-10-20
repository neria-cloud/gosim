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
		linkCount: f.linkCount, // XXX: test that link count is maintained
	}
}

type backingDir struct {
	inode   int
	parent  int
	name    string // name in parent
	entries map[string]int
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
				entries: map[string]int{
					"etc": 2,
					"app": 4,
				},
			},
			2: &backingDir{
				inode:  2,
				parent: RootInode,
				name:   "etc",
				entries: map[string]int{
					"hosts": 3,
				},
			},
			3: &backingFile{
				inode: 3,
				file: chunkedFileFromBytes([]byte(`# hosts
127.0.0.1 localhost
`)),
				linkCount: 1,
			},
			4: &backingDir{
				inode:   4,
				parent:  RootInode,
				name:    "app",
				entries: make(map[string]int),
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
}

func (o *allocOp) apply(fs *filesystemState, gcer gcer) {
	if _, ok := fs.objects[o.inode]; ok {
		panic("double alloc")
	}
	if o.dir {
		fs.objects[o.inode] = &backingDir{
			inode:   o.inode,
			entries: make(map[string]int),
		}
	} else {
		fs.objects[o.inode] = &backingFile{
			inode: o.inode,
			file:  &chunkedFile{},
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

func (fs *Filesystem) Write(inode int, pos int64, buffer abi.ByteView) int {
	file := fs.mem.getFile(inode)

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

	return len
}

func (fs *Filesystem) Truncate(inode, pos int) {
	file := fs.mem.getFile(inode)

	if pos != file.len() {
		op := &resizeOp{
			inode: inode,
			size:  pos,
		}
		fs.apply(op)
	}
}

func (fs *Filesystem) Sync(inode int) {
	fs.flushInode(inode)
}

func (fs *Filesystem) Getdirinode(dirInode int, path string) (int, error) {
	// For chdir
	dirInode, _, err := fs.walkpath(dirInode, path, false)
	if err != nil {
		return 0, err
	}

	_, ok := fs.mem.getDir(dirInode)
	if !ok {
		return 0, syscall.ENOTDIR
	}

	return dirInode, nil
}

func (fs *Filesystem) Mkdir(dirInode int, path string) error {
	dirInode, name, err := fs.walkpath(dirInode, path, true)
	if err != nil {
		return err
	}

	dir, ok := fs.mem.getDir(dirInode)
	if !ok {
		return syscall.ENOTDIR
	}

	if _, ok := dir.entries[name]; ok {
		return syscall.EEXIST
	}

	inode := fs.mem.next
	op := &allocOp{
		dir:   true,
		inode: inode,
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
}

func (fs *Filesystem) statLocked(inode int) (StatResp, error) {
	obj := fs.mem.objects[inode]
	switch obj := obj.(type) {
	case *backingDir:
		return StatResp{
			Inode: inode,
			IsDir: true,
		}, nil

	case *backingFile:
		return StatResp{
			Inode: inode,
			IsDir: false,
			Size:  int64(obj.file.size),
		}, nil

	default:
		panic("invalid")
	}
}

func (fs *Filesystem) Stat(dirInode int, path string) (StatResp, error) {
	dirInode, name, err := fs.walkpath(dirInode, path, true)
	if err != nil {
		return StatResp{}, err
	}

	dir, ok := fs.mem.getDir(dirInode)
	if !ok {
		return StatResp{}, syscall.ENOTDIR
	}

	var inode int
	if name == "." {
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

func (fs *Filesystem) Remove(dirInode int, path string, rmDir bool) error {
	dirInode, name, err := fs.walkpath(dirInode, path, true)
	if err != nil {
		return err
	}

	dir, ok := fs.mem.getDir(dirInode)
	if !ok {
		return syscall.ENOTDIR
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

func (fs *Filesystem) Rename(fromInode int, fromPath string, toInode int, toPath string) error {
	fromInode, fromName, err := fs.walkpath(fromInode, fromPath, true)
	if err != nil {
		return err
	}

	fromDir, ok := fs.mem.getDir(fromInode)
	if !ok {
		return syscall.ENOTDIR
	}

	inode, ok := fromDir.entries[fromName]
	if !ok {
		return syscall.ENOENT
	}

	toInode, toName, err := fs.walkpath(toInode, toPath, true)
	if err != nil {
		return err
	}

	if _, ok := fs.mem.getDir(toInode); !ok {
		return syscall.ENOTDIR
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

func (fs *Filesystem) OpenFile(dirInode int, path string, flag int) (int, error) {
	dirInode, name, err := fs.walkpath(dirInode, path, true)
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
	if name != "." {
		inode, ok = dir.entries[name]
		if !ok {
			if flag&os.O_CREATE == 0 {
				return 0, syscall.ENOENT
			}

			inode = fs.mem.next
			op := &allocOp{
				dir:   false,
				inode: inode,
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

			// slog.Info("fs create", "path", args.Path, "inode", inode)
		}
	} else {
		// janky...
		// TODO: handle "." everywhere that reads dir, and do it cleanly
		// TODO: handle "..",
		inode = dirInode
		ok = true
	}

	if ok && flag&os.O_EXCL != 0 {
		return 0, syscall.EEXIST
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

func (fs *Filesystem) ReadDir(inode int) ([]ReadDirEntry, error) {
	dir, ok := fs.mem.getDir(inode)
	if !ok {
		return nil, syscall.ENOTDIR
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

func (fs *Filesystem) walkpath(baseInode int, path string, keepLast bool) (inode int, entry string, err error) {
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
			return inode, name, nil
		}

		if name == "" {
			// entire path is empty? assume current dir
			return inode, "", nil
		}

		// check it is a dir
		dir, ok := fs.mem.getDir(inode)
		if !ok {
			return 0, "", syscall.ENOTDIR
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
