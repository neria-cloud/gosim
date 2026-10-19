//go:build gosim_retired

package go125

import "github.com/neria-cloud/gosim/rt/runtime"

func HashMaphash_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}
