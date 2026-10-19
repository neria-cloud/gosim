//go:build sim

package rt

import (
	"time"

	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func SetTimeout(timeout time.Duration) error {
	_, err := control.Call(kernel.SetTimeoutRequest{TimeoutNS: int64(timeout)}, kernel.DecodeSetTimeoutResult)
	return err
}
