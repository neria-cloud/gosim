package kernel

import "github.com/neria-cloud/gosim/internal/contracts"

// TODO(migration): add the generated-envelope invocation transport in Task 6.
// This lifecycle-only coordinator has no callable syscall fallback.
type Dispatcher struct{ kernel *Kernel }

func NewDispatcher(kernel *Kernel) *Dispatcher { return &Dispatcher{kernel: kernel} }
func (d *Dispatcher) CancelEpoch(epoch contracts.EpochRef) {
	for id, wait := range d.kernel.waits {
		if wait.caller == epoch {
			delete(d.kernel.waits, id)
		}
	}
}

func (d *Dispatcher) Drain() error {
	if len(d.kernel.waits) != 0 {
		return contracts.ErrInvalidState
	}
	return nil
}
