//go:build sim

package rt

import (
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func Info() RunInfo {
	r, err := control.Call(kernel.InfoRequest{}, kernel.DecodeInfoResult)
	if err != nil {
		panic(err)
	}
	return RunInfo(r)
}
