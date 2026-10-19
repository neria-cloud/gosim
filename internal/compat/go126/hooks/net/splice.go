//go:build go1.26 && !go1.27

package net

import (
	"unsafe"

	kh "github.com/neria-cloud/gosim/internal/compat/go126/hooks/kernel"
	"github.com/neria-cloud/gosim/internal/contracts"
)

// InternalPoll_Splice declines the Linux pipe optimization. io.Copy then uses
// guarded socket reads/writes, without allocating host pipes or descriptors.
func InternalPoll_Splice(_, _ unsafe.Pointer, _ int64) (int64, bool, error) {
	kh.Require(contracts.Net, "splice")
	return 0, false, nil
}

// InternalPoll_SendFile likewise leaves source position and destination intact.
func InternalPoll_SendFile(_ unsafe.Pointer, _ uintptr, _ int64) (int64, error, bool) {
	kh.Require(contracts.Net, "sendfile")
	return 0, nil, false
}
