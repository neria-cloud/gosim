//go:build sim

package gosim

import (
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func IsSim() bool             { return true }
func CurrentMachine() Machine { ref := control.CurrentRef(); return Machine{ref.RunID, ref.ID} }
func (m Machine) ref() contracts.MachineRef {
	return contracts.MachineRef{RunID: m.runID, ID: m.machineID}
}

func NewMachine(c MachineConfig) (Machine, error) {
	r, err := control.Call(kernel.CreateRequest{Label: c.Label, Main: c.Main}, kernel.DecodeCreateResult)
	return Machine{r.Machine.RunID, r.Machine.ID}, err
}

func (m Machine) Label() (string, error) {
	r, err := control.Call(kernel.LabelRequest{Machine: m.ref()}, kernel.DecodeLabelResult)
	return r.Label, err
}

func (m Machine) State() (MachineState, error) {
	r, err := control.Call(kernel.StateRequest{Machine: m.ref()}, kernel.DecodeStateResult)
	return MachineState(r.State), err
}

func (m Machine) Start() error {
	_, err := control.Call(kernel.StartRequest{Machine: m.ref()}, kernel.DecodeStartResult)
	return err
}

func (m Machine) Stop() error {
	_, err := control.Call(kernel.StopRequest{Machine: m.ref(), Kind: contracts.StopGraceful}, kernel.DecodeStopResult)
	return err
}

func (m Machine) Crash() error {
	_, err := control.Call(kernel.StopRequest{Machine: m.ref(), Kind: contracts.StopCrash}, kernel.DecodeStopResult)
	return err
}

func (m Machine) Restart() error {
	_, err := control.Call(kernel.RestartRequest{Machine: m.ref()}, kernel.DecodeRestartResult)
	return err
}

func (m Machine) SetMain(main func()) error {
	_, err := control.Call(kernel.SetMainRequest{Machine: m.ref(), Main: main}, kernel.DecodeSetMainResult)
	return err
}

func (m Machine) Wait() (Exit, error) {
	r, err := control.Call(kernel.WaitRequest{Machine: m.ref()}, kernel.DecodeWaitResult)
	return Exit{r.Exit.Epoch, ExitKind(r.Exit.Kind)}, err
}

func (m Machine) Destroy() error {
	_, err := control.Call(kernel.DestroyRequest{Machine: m.ref()}, kernel.DecodeDestroyResult)
	return err
}

func ExitSelf() {
	_, err := control.Call(kernel.ExitSelfRequest{Kind: contracts.StopGraceful}, kernel.DecodeExitSelfResult)
	panic(err)
}

func CrashSelf() {
	_, err := control.Call(kernel.ExitSelfRequest{Kind: contracts.StopCrash}, kernel.DecodeExitSelfResult)
	panic(err)
}
