//go:build go1.25 && !go1.26

package rt

import (
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func Maps_clone(m any) any {
	return gosimruntime.CloneMap(m)
}
