package contracts

type (
	FSConfig      struct{ WorkingDir string }
	FSFaultConfig struct{ Probability, BeforeProbability float64 }
	InodeInfo     struct {
		MemLinks     int
		MemExists    bool
		DiskLinks    int
		DiskExists   bool
		Handles, Ops int
	}
)

type FSControl interface {
	ConfigureMachine(MachineRef, FSConfig) error
	SetCrashMode(MachineRef, uint8) error
	SetSyncFault(MachineRef, FSFaultConfig) error
	InspectInode(MachineRef, int) (InodeInfo, error)
	Enumerate(MachineRef, uint64) (uint64, error)
	NextSnapshot(uint64) (uint64, bool, error)
	CloseIterator(uint64) error
	Restore(MachineRef, uint64) error
	CloseSnapshot(uint64) error
}
