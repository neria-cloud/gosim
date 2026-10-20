//go:build go1.25 && !go1.26

package kernel

import (
	"syscall"
	"unsafe"

	k "github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// InternalPoll_writev submits one ordered write through the descriptor owner.
// Flattening keeps all application views borrowed only for this invocation and
// preserves partial-write reporting for internal/poll's existing retry loop.
func InternalPoll_writev(fd int, vectors []syscall.Iovec) (uintptr, error) {
	if len(vectors) > maxWritevVectors {
		return 0, syscall.EINVAL
	}
	total := uint64(0)
	for _, v := range vectors {
		if v.Len > uint64(^uint(0)>>1)-total {
			return 0, syscall.EINVAL
		}
		if v.Len != 0 && v.Base == nil {
			return 0, syscall.EFAULT
		}
		total += v.Len
	}
	data := make([]byte, 0, int(total))
	for _, v := range vectors {
		simrt.RaceReadRange(unsafe.Pointer(v.Base), int(v.Len))
		data = append(data, unsafe.Slice(v.Base, int(v.Len))...)
	}
	result, err := Call(k.WriteRequest{FD: fd, From: abi.ByteView{Ptr: data}}, k.DecodeWriteResult)
	return uintptr(result.Count), err
}
