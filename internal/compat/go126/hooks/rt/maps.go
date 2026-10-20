//go:build go1.26 && !go1.27

package rt

import (
	"github.com/neria-cloud/gosim/rt/runtime"
)

func Maps_clone(m any) any {
	return gosimruntime.CloneMap(m)
}
