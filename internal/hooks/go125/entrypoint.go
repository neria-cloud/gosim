package go125

import (
	"github.com/neria-cloud/gosim/internal/simulation"
	"github.com/neria-cloud/gosim/internal/simulation/syscallabi"
	"github.com/neria-cloud/gosim/internal/testing"
	"github.com/neria-cloud/gosim/rt/runtime"
)

func Runtime() gosimruntime.Runtime {
	return runtimeImpl{}
}

type runtimeImpl struct{}

var _ gosimruntime.Runtime = runtimeImpl{}

func (r runtimeImpl) Run(fn func()) {
	simulation.Runtime(fn)
}

func (r runtimeImpl) Setup() {
	syscallabi.Setup()
}

func (r runtimeImpl) TestEntrypoint(match string, skip string, tests []gosimruntime.Test) bool {
	return testing.Entrypoint(match, skip, tests)
}
