//go:build !sim

package gosim

func IsSim() bool                               { return false }
func CurrentMachine() Machine                   { return Machine{} }
func NewMachine(MachineConfig) (Machine, error) { return Machine{}, ErrNotSim }
func (Machine) Label() (string, error)          { return "", ErrNotSim }
func (Machine) State() (MachineState, error)    { return 0, ErrNotSim }
func (Machine) Start() error                    { return ErrNotSim }
func (Machine) Stop() error                     { return ErrNotSim }
func (Machine) Crash() error                    { return ErrNotSim }
func (Machine) Restart() error                  { return ErrNotSim }
func (Machine) SetMain(func()) error            { return ErrNotSim }
func (Machine) Wait() (Exit, error)             { return Exit{}, ErrNotSim }
func (Machine) Destroy() error                  { return ErrNotSim }
func ExitSelf()                                 { panic(ErrNotSim) }
func CrashSelf()                                { panic(ErrNotSim) }
