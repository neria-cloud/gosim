package go125

import "github.com/neria-cloud/gosim/gosimruntime"

func Net_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}
