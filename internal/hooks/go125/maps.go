//go:build gosim_retired

package go125

import (
	"github.com/neria-cloud/gosim/rt/runtime"
)

func Maps_clone(m any) any {
	return gosimruntime.CloneMap(m)
}
