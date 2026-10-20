package kernel

import (
	"github.com/neria-cloud/gosim/internal/contracts"
)

func (k *Kernel) Network(ctx contracts.CallContext) (contracts.SocketOps, error) {
	k.Require(ctx, contracts.Net, "network syscall")
	run, err := k.activeEpoch(ctx.Epoch)
	if err != nil {
		return nil, err
	}
	if run.bindings.Sockets == nil {
		return nil, contracts.ErrInvalidState
	}
	return run.bindings.Sockets, nil
}
