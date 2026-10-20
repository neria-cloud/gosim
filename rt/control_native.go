//go:build !sim

package rt

import (
	"time"

	"github.com/neria-cloud/gosim"
)

func SetTimeout(time.Duration) error { return gosim.ErrNotSim }
