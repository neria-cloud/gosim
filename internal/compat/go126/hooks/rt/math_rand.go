//go:build go1.26 && !go1.27

package rt

import gosimruntime "github.com/neria-cloud/gosim/rt/runtime"

func MathRand_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}
