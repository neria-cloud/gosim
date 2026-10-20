//go:build go1.25 && !go1.26

package rt

import (
	"unsafe"

	"github.com/neria-cloud/gosim/rt/runtime"
)

func HashMaphash_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}

// HashMaphash_runtime_memhash models byte hashing with a seed-dependent FNV-1a
// state. Host runtime.memhash uses a process-random AES key and cannot replay.
func HashMaphash_runtime_memhash(p unsafe.Pointer, seed, size uintptr) uintptr {
	h := uint64(seed) ^ 14695981039346656037
	for _, b := range unsafe.Slice((*byte)(p), size) {
		h = (h ^ uint64(b)) * 1099511628211
	}
	return uintptr(h)
}
