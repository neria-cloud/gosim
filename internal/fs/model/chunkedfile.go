package model

import (
	"slices"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// A chunkedFile is the backing storage for files. It supports efficient reads
// and writes with multiple versions of the file existing by storing the data in
// fixed-sized reference-counted copy-on-write chunks.
//
// The goals are as follows:
//
// To simulate crashes we store two versions of each file: The latest in-memory
// representation with all writes faithfully applied, and the on-disk version
// that does not yet have all writes. We want to share storage and work as much
// as possible between these two versions.
//
// When a file gets modified, we would like to allocate as little memory as
// possible. Common scenarios are resizing files, appending to a file, and
// rewriting parts of a file.
//
// Tricky combinations of these requirements are, for example, rewriting a small
// part of a large file. Do we make a copy of the entire file and modify just
// the small part?  When we append to a file and its backing storage needs to
// grow, do we copy all existing data?
//
// To support these operations efficiently, the chunkedFile stores data in
// fixed-sized reference-counted chunks: Each file is split in chunks of
// chunkSize, and all operations modify those chunks. When copying a large file,
// we share the chunks. If some chunks get modified later, only those get
// copied. When we append to a file, we similarly keep all the old chunks.
//
// When a write comes in over a syscall, we read the data into correctly-aligned
// chunks. In the happy case, that is the only time data is copied byte-by-byte
// and afterwards we only move references to the chunk around.
//
// Chunks belong to model instances and retained snapshots. No allocation pool
// shares mutable storage across simulation runs.
type chunkedFile struct {
	// TODO: inline a fixed size array of chunks that covers most file sizes?
	chunks []*refCountedChunk
	size   int
}

const chunkSize = 1024

func (w *chunkedFile) AllocatedBlocks() uint64 {
	var count uint64
	for _, chunk := range w.chunks {
		if chunk != nil {
			count++
		}
	}
	return count
}

func (w *chunkedFile) NewBlocksForWrite(pos, size int) uint64 {
	if size == 0 {
		return 0
	}
	first, last := pos/chunkSize, (pos+size-1)/chunkSize
	var count uint64
	for i := first; i <= last; i++ {
		if i >= len(w.chunks) || w.chunks[i] == nil {
			count++
		}
	}
	return count
}

type refCountedChunk struct {
	// XXX: inline data to skip an object?
	refs int
	data []byte
}

func chunkedFileFromBytes(data []byte) *chunkedFile {
	chunks := make([]*refCountedChunk, 0, (len(data)+chunkSize-1)/chunkSize)
	tail := data
	for len(tail) > 0 {
		chunk := allocRefCountedChunk()
		chunk.refs = 1
		n := copy(chunk.data, tail)
		tail = tail[n:]
		chunks = append(chunks, chunk)
	}
	return &chunkedFile{
		chunks: chunks,
		size:   len(data),
	}
}

func allocRefCountedChunk() *refCountedChunk {
	return &refCountedChunk{refs: 1, data: make([]byte, chunkSize)}
}

func (c *refCountedChunk) incRef() {
	if c.refs <= 0 || c.refs == int(^uint(0)>>1) {
		panic("help")
	}
	c.refs++
}

func (c *refCountedChunk) decRef() {
	if c.refs <= 0 {
		panic("help")
	}
	c.refs--
	if c.refs == 0 {
		c.data = nil
	}
}

// TODO: when do we free files? need to track open inodes. (should be not too
// bad? close does get called?)

// TODO: when do we free the filesystem? need to track alive machines. (can do
// this at end of simulation?)

// small helper to make testing possible
func requiredCount(chunkPos int, len int) int {
	if len == 0 {
		return 0
	}
	count := 0
	if chunkPos != 0 {
		count++
		len -= min(chunkSize-chunkPos, len)
	}
	count += (len + chunkSize - 1) / chunkSize
	return count
}

func makeChunks(pos int, view abi.ByteView) []*refCountedChunk {
	if view.Len() == 0 {
		return nil
	}
	chunkPos := pos % chunkSize
	count := requiredCount(chunkPos, view.Len())
	out := make([]*refCountedChunk, 0, count)
	if chunkPos != 0 {
		chunk := allocRefCountedChunk()
		n := view.Read(chunk.data[:chunkSize-chunkPos])
		view = view.SliceFrom(n)
		out = append(out, chunk)
	}
	for view.Len() > 0 {
		chunk := allocRefCountedChunk()
		n := view.Read(chunk.data)
		view = view.SliceFrom(n)
		out = append(out, chunk)
	}
	return out
}

// TODO: add initializer?

var zeroChunk = &refCountedChunk{
	refs: -1e6, // should trigger asserts hopefully
	data: make([]byte, chunkSize),
}

func (w *chunkedFile) Resize(newSize int) {
	oldCount := len(w.chunks)
	newCount := (newSize + chunkSize - 1) / chunkSize

	if newCount > oldCount {
		w.chunks = append(w.chunks, make([]*refCountedChunk, newCount-oldCount)...)
	} else {
		for i := newCount; i < oldCount; i++ {
			w.releaseChunk(i)
		}
		// TODO: free memory? unclear
		w.chunks = w.chunks[:newCount]
	}

	// TODO: document guarantees about zeroing?

	// zero tail of last chunk if needed
	oldSize := w.size
	if newSize < oldSize {
		if chunkPos := newSize % chunkSize; chunkPos != 0 {
			if chunkIdx := newCount - 1; w.chunks[chunkIdx] != nil {
				chunk := w.ensureWritableChunk(chunkIdx)
				// TODO: only have to zero up to oldSize, really?
				copy(chunk.data[chunkPos:], zeroes)
			}
		}
	}

	// TODO: help zero (part) of chunk?
	w.size = newSize
}

func (w *chunkedFile) releaseChunk(idx int) {
	chunk := w.chunks[idx]
	if chunk != nil {
		chunk.decRef()
		w.chunks[idx] = nil
	}
}

func (w *chunkedFile) ensureWritableChunk(idx int) *refCountedChunk {
	if chunk := w.chunks[idx]; chunk != nil && chunk.refs == 1 {
		return chunk
	}
	oldChunk := w.getReadableChunk(idx)
	newChunk := allocRefCountedChunk()
	copy(newChunk.data, oldChunk.data)
	w.releaseChunk(idx)
	w.chunks[idx] = newChunk
	return newChunk
}

func (w *chunkedFile) getReadableChunk(idx int) *refCountedChunk {
	chunk := w.chunks[idx]
	if chunk == nil {
		return zeroChunk
	}
	return chunk
}

func (w *chunkedFile) Clone() *chunkedFile {
	chunks := slices.Clone(w.chunks)
	for _, chunk := range chunks {
		if chunk != nil {
			chunk.incRef()
		}
	}
	return &chunkedFile{
		chunks: chunks,
		size:   w.size,
	}
}

func (w *chunkedFile) Free() {
	for i, chunk := range w.chunks {
		if chunk != nil {
			chunk.decRef()
		}
		w.chunks[i] = nil
	}
	// set to nil to prevent reuse
	w.chunks = nil
	w.size = 0
}

func (w *chunkedFile) Write(pos int, size int, chunks []*refCountedChunk) {
	if size == 0 {
		// TODO: is this reasonable?
		return
	}

	// partial chunks on either end
	chunkIdx := pos / chunkSize
	if chunkPos := pos % chunkSize; chunkPos != 0 {
		// TODO: what if this partial on both sides? handle and test.
		chunk := w.ensureWritableChunk(chunkIdx)
		n := copy(chunk.data[chunkPos:min(chunkPos+size, chunkSize)], chunks[0].data)
		chunks = chunks[1:]
		size -= n
		chunkIdx++
	}
	for size >= chunkSize {
		w.releaseChunk(chunkIdx)
		chunks[0].incRef()
		w.chunks[chunkIdx] = chunks[0]
		chunks = chunks[1:]
		size -= chunkSize
		chunkIdx++
	}
	if size > 0 {
		chunk := w.ensureWritableChunk(chunkIdx)
		copy(chunk.data[:size], chunks[0].data)
	}
}

func (w *chunkedFile) Read(pos int, out abi.ByteView) {
	// TODO: precondition that read fits in file?

	chunkIdx := pos / chunkSize
	if chunkPos := pos % chunkSize; chunkPos != 0 {
		// TODO: what if this partial on both sides? handle and test.
		chunk := w.getReadableChunk(chunkIdx)
		n := out.Write(chunk.data[chunkPos:])
		out = out.SliceFrom(n)
		chunkIdx++
	}
	for out.Len() > 0 {
		chunk := w.getReadableChunk(chunkIdx)
		n := out.Write(chunk.data)
		out = out.SliceFrom(n)
		chunkIdx++
	}
}
