//go:build go1.25 && !go1.26

package rt

import gosimruntime "github.com/neria-cloud/gosim/rt/runtime"

func MathRand_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}
