package go125

import (
	"github.com/neria-cloud/gosim/gosimruntime"
)

func Maps_clone(m any) any {
	return gosimruntime.CloneMap(m)
}
