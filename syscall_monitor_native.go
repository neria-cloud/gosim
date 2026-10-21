//go:build !sim

package gosim

func (syscallsMonitor) Start() error                           { return ErrNotSim }
func (syscallsMonitor) Stop() error                            { return ErrNotSim }
func (syscallsMonitor) Reset() error                           { return ErrNotSim }
func (syscallsMonitor) Counts() (map[string]uint64, error)     { return nil, ErrNotSim }
func (syscallsMonitor) Stats() (map[string]SyscallStat, error) { return nil, ErrNotSim }
func (syscallsMonitor) List() ([]SyscallInfo, error)           { return nil, ErrNotSim }
