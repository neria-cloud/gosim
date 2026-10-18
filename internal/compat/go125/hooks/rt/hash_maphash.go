//go:build go1.25 && !go1.26

package rt

import "github.com/neria-cloud/gosim/rt/runtime"

func HashMaphash_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}
