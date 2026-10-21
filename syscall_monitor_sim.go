//go:build sim

package gosim

import (
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func (m syscallsMonitor) setCounting(enabled bool) error {
	_, err := control.Call(kernel.SetSyscallCountingRequest{
		Machine: m.machine.ref(), Enabled: enabled,
	}, kernel.DecodeSetSyscallCountingResult)
	return err
}

func (m syscallsMonitor) Start() error { return m.setCounting(true) }
func (m syscallsMonitor) Stop() error  { return m.setCounting(false) }

func (m syscallsMonitor) Reset() error {
	_, err := control.Call(kernel.ResetSyscallStatsRequest{Machine: m.machine.ref()}, kernel.DecodeResetSyscallStatsResult)
	return err
}

func (m syscallsMonitor) snapshot() ([]contracts.SyscallRecord, error) {
	out, err := control.Call(kernel.SyscallStatsRequest{Machine: m.machine.ref()}, kernel.DecodeSyscallStatsResult)
	if err != nil {
		return nil, err
	}
	return out.Records, nil
}

func (m syscallsMonitor) Counts() (map[string]uint64, error) {
	rows, err := m.snapshot()
	if err != nil {
		return nil, err
	}
	result := make(map[string]uint64)
	for _, row := range rows {
		result[row.Name] += row.Count
	}
	return result, nil
}

func (m syscallsMonitor) Stats() (map[string]SyscallStat, error) {
	rows, err := m.snapshot()
	if err != nil {
		return nil, err
	}
	result := make(map[string]SyscallStat)
	for _, row := range rows {
		stat, exists := result[row.Name]
		if !exists {
			stat = SyscallStat{Group: SyscallGroup(row.Group), ByDescriptor: make(map[DescriptorGroup]uint64)}
		}
		stat.Count += row.Count
		stat.ByDescriptor[DescriptorGroup(row.DescriptorGroup)] += row.Count
		result[row.Name] = stat
	}
	return result, nil
}

// CountableSyscalls returns available syscall names and static metadata sorted by Name.
// The dictionary depends only on the simulation feature set, not on a machine.
// The returned slice is detached from the catalogue.
func CountableSyscalls() ([]SyscallInfo, error) {
	out, err := control.Call(kernel.CountableSyscallsRequest{}, kernel.DecodeCountableSyscallsResult)
	if err != nil {
		return nil, err
	}
	result := make([]SyscallInfo, 0, len(out.Syscalls))
	for _, entry := range out.Syscalls {
		result = append(result, SyscallInfo{Name: entry.Name, Group: SyscallGroup(entry.Group), DescriptorBased: entry.DescriptorBased})
	}
	slices.SortFunc(result, func(a, b SyscallInfo) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}
