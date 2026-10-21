//go:build go1.25

package rt

import "github.com/neria-cloud/gosim/rt/runtime"

func MathRandV2_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}
