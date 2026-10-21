package gosim

// SyscallGroup is a syscall's fixed functional group, independent of its FD.
type SyscallGroup string

const (
	SyscallGroupRT  SyscallGroup = "rt"
	SyscallGroupFS  SyscallGroup = "fs"
	SyscallGroupNet SyscallGroup = "net"
)

// DescriptorGroup describes the primary input descriptor's owner at admission.
type DescriptorGroup string

const (
	DescriptorGroupNone    DescriptorGroup = "none" // no primary input descriptor
	DescriptorGroupRT      DescriptorGroup = "rt"
	DescriptorGroupFS      DescriptorGroup = "fs"
	DescriptorGroupNet     DescriptorGroup = "net"
	DescriptorGroupUnknown DescriptorGroup = "unknown" // descriptor lookup failed
)

// SyscallStat contains a total and its nonzero descriptor-owner buckets.
// Count equals the sum of ByDescriptor. Group is fixed even for mixed owners.
type SyscallStat struct {
	Count        uint64
	Group        SyscallGroup
	ByDescriptor map[DescriptorGroup]uint64
}

// SyscallInfo describes an available countable name, even if it has not run.
// DescriptorBased means the route consumes an existing primary input FD;
// directory-context and returned descriptors are excluded.
type SyscallInfo struct {
	Name            string
	Group           SyscallGroup
	DescriptorBased bool
}

// SyscallsMonitor controls one machine's routed syscall-attempt counters.
// Counting starts disabled. State survives machine stop, crash, and restart.
// All methods validate the machine handle; native methods return ErrNotSim.
// Calls are scheduling points, so stop counting or use a workload barrier when
// comparing separate snapshots. Native calls and hook-local errors are excluded.
type SyscallsMonitor interface {
	// Start enables counting without clearing existing statistics.
	Start() error
	// Stop pauses counting without clearing existing statistics.
	Stop() error
	// Reset clears statistics without changing whether counting is enabled.
	Reset() error
	// Counts returns an owned map of observed syscall names to nonzero totals.
	Counts() (map[string]uint64, error)
	// Stats returns an owned snapshot, including fresh nested descriptor maps.
	Stats() (map[string]SyscallStat, error)
}

type syscallsMonitor struct{ machine Machine }

// SyscallsMonitor returns a non-nil handle without starting or enabling anything.
// Handles for the same machine share state and become invalid after Destroy.
func (m Machine) SyscallsMonitor() SyscallsMonitor { return syscallsMonitor{machine: m} }

var _ SyscallsMonitor = syscallsMonitor{}
