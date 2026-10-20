package gosim

// Machine is a control reference scoped to one simulation run. Start publishes
// the first epoch; Restart preserves provider state and initializes fresh globals.
// A machine's return or explicit termination cancels its goroutines without
// running their pending defers.
type (
	Machine      struct{ runID, machineID uint64 }
	MachineState uint8
)

const (
	New MachineState = iota
	Starting
	Running
	Stopping
	Stopped
	Crashed
)

type ExitKind uint8

const (
	Graceful ExitKind = iota
	Crash
)

type Exit struct {
	Epoch uint64
	Kind  ExitKind
}

// MachineConfig is copied by NewMachine. Main must be non-nil. The new machine
// is unstarted so optional providers can be configured before Start.
type MachineConfig struct {
	Label string
	Main  func()
}

func (m Machine) ID() uint64    { return m.machineID }
func (m Machine) RunID() uint64 { return m.runID }
