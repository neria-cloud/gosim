//go:build go1.25

package rt

import (
	"github.com/neria-cloud/gosim/rt/runtime"
)

func Maps_clone(m any) any {
	return gosimruntime.CloneMap(m)
}
