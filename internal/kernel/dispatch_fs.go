package kernel

import (
	"errors"

	"github.com/neria-cloud/gosim/internal/contracts"
)

func (k *Kernel) Filesystem(ctx contracts.CallContext) (contracts.FileOps, error) {
	k.Require(ctx, contracts.FS, "filesystem syscall")
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return nil, err
	}
	if run.bindings.Files == nil {
		return nil, errors.New("filesystem service missing")
	}
	return run.bindings.Files, nil
}
