package kernel

import (
	"math"

	"github.com/neria-cloud/gosim/internal/contracts"
)

type syscallDescriptor struct {
	name  string
	group contracts.Feature
}

// syscallDataSyncSlot splits Sync's variants without adding a dispatchable route.
const syscallDataSyncSlot RouteID = routeLimit

// Holes are deliberately uncounted. FcntlLock shares FcntlInt's canonical slot.
var syscallCatalogue = [...]syscallDescriptor{
	RouteRead:           {"read", contracts.RT},
	RouteWrite:          {"write", contracts.RT},
	RouteWritev:         {"writev", contracts.RT},
	RouteClose:          {"close", contracts.RT},
	RouteFcntlInt:       {"fcntl", contracts.RT},
	RouteGetrandom:      {"getrandom", contracts.RT},
	RouteGetpid:         {"getpid", contracts.RT},
	RouteUname:          {"uname", contracts.RT},
	RouteOpenat:         {"openat", contracts.FS},
	RouteRenameat:       {"renameat", contracts.FS},
	RouteGetdents64:     {"getdents64", contracts.FS},
	RouteFallocate:      {"fallocate", contracts.FS},
	RouteSeek:           {"lseek", contracts.FS},
	RoutePwrite:         {"pwrite64", contracts.FS},
	RoutePread:          {"pread64", contracts.FS},
	RouteSync:           {"fsync", contracts.FS},
	syscallDataSyncSlot: {"fdatasync", contracts.FS},
	RouteFstat:          {"fstat", contracts.FS},
	RouteStatat:         {"fstatat", contracts.FS},
	RouteUnlinkat:       {"unlinkat", contracts.FS},
	RouteTruncate:       {"ftruncate", contracts.FS},
	RouteChdir:          {"chdir", contracts.FS},
	RouteGetcwd:         {"getcwd", contracts.FS},
	RouteMkdirat:        {"mkdirat", contracts.FS},
	RouteFlock:          {"flock", contracts.FS},
	RouteMap:            {"mmap", contracts.FS},
	RouteUnmap:          {"munmap", contracts.FS},
	RouteAdvise:         {"madvise", contracts.FS},
	RouteStatFS:         {"statfs", contracts.FS},
	RouteFstatFS:        {"fstatfs", contracts.FS},
	RouteFchmod:         {"fchmod", contracts.FS},
	RouteChmodat:        {"fchmodat", contracts.FS},
	RouteFchown:         {"fchown", contracts.FS},
	RouteChownat:        {"fchownat", contracts.FS},
	RouteSocket:         {"socket", contracts.Net},
	RouteBind:           {"bind", contracts.Net},
	RouteListen:         {"listen", contracts.Net},
	RouteAccept:         {"accept4", contracts.Net},
	RouteConnect:        {"connect", contracts.Net},
	RouteLocalAddress:   {"getsockname", contracts.Net},
	RoutePeerAddress:    {"getpeername", contracts.Net},
	RouteSetOption:      {"setsockopt", contracts.Net},
	RouteGetOption:      {"getsockopt", contracts.Net},
}

type syscallObservation struct {
	slot            RouteID
	descriptorGroup contracts.DescriptorGroup
}

type syscallCounter struct {
	total        uint64
	byDescriptor [contracts.DescriptorGroupLimit]uint64
}

type syscallCounters struct {
	enabled bool
	values  [len(syscallCatalogue)]syscallCounter
}

func syscallCounterSlot(inv *Invocation) RouteID {
	if inv.Route == RouteFcntlLock {
		return RouteFcntlInt
	}
	if inv.Route == RouteSync && inv.Args[0].(SyncRequest).DataOnly {
		return syscallDataSyncSlot
	}
	return inv.Route
}

func syscallInfo(slot RouteID) contracts.SyscallInfo {
	metadata := syscallCatalogue[slot]
	route := slot
	if slot == syscallDataSyncSlot {
		route = RouteSync
	}
	return contracts.SyscallInfo{
		Name: metadata.name, Group: featureName(metadata.group),
		DescriptorBased: syscallRouteHasFD(route),
	}
}

func descriptorGroupName(group contracts.DescriptorGroup) string {
	switch group {
	case contracts.DescriptorNone:
		return "none"
	case contracts.DescriptorRT:
		return "rt"
	case contracts.DescriptorFS:
		return "fs"
	case contracts.DescriptorNet:
		return "net"
	case contracts.DescriptorUnknown:
		return "unknown"
	default:
		panic("invalid internal descriptor group")
	}
}

// observeSyscall requires an admitted epoch and a validated typed payload.
// Capture before execution, especially before close removes its descriptor.
// TODO(not planned): per-FD identity, paths, result descriptors, and multi-FD
// attribution need another data model; this observation retains only ownership.
//
//go:norace
func (k *Kernel) observeSyscall(inv *Invocation) (syscallObservation, bool) {
	slot := syscallCounterSlot(inv)
	if slot >= RouteID(len(syscallCatalogue)) {
		return syscallObservation{}, false
	}
	info := syscallCatalogue[slot]
	if info.name == "" || !k.features.Has(info.group) {
		return syscallObservation{}, false
	}
	observation := syscallObservation{slot: slot, descriptorGroup: contracts.DescriptorNone}
	fd, hasFD := syscallInputFD(inv)
	if !hasFD {
		return observation, true
	}
	machine := k.machineByID[inv.Context.Epoch.Machine.ID]
	resource, err := machine.current.descriptors.Lookup(fd)
	if err != nil {
		observation.descriptorGroup = contracts.DescriptorUnknown
		return observation, true
	}
	switch resource.Owner() {
	case contracts.RT:
		observation.descriptorGroup = contracts.DescriptorRT
	case contracts.FS:
		observation.descriptorGroup = contracts.DescriptorFS
	case contracts.Net:
		observation.descriptorGroup = contracts.DescriptorNet
	default:
		k.invariant("invalid syscall descriptor owner")
		return syscallObservation{}, false
	}
	return observation, true
}

//go:norace
func (k *Kernel) countSyscall(inv *Invocation) {
	machine := k.machineByID[inv.Context.Epoch.Machine.ID]
	if !machine.syscalls.enabled {
		return
	}
	observation, ok := k.observeSyscall(inv)
	if !ok {
		return
	}
	// TODO(not planned): a future collection predicate belongs here, before
	// aggregation; resolve static metadata through the slot without event maps.
	counter := &machine.syscalls.values[observation.slot]
	if counter.total == math.MaxUint64 {
		k.invariant("syscall counter overflow")
		return
	}
	counter.total++
	counter.byDescriptor[observation.descriptorGroup]++
}

//go:norace
func (k *Kernel) SetSyscallCounting(ref contracts.MachineRef, enabled bool) error {
	machine, err := k.Lookup(ref)
	if err != nil {
		return err
	}
	machine.syscalls.enabled = enabled
	return nil
}

//go:norace
func (k *Kernel) ResetSyscallStats(ref contracts.MachineRef) error {
	machine, err := k.Lookup(ref)
	if err != nil {
		return err
	}
	clear(machine.syscalls.values[:])
	return nil
}

// SyscallStats copies nonzero bucket rows in slot/descriptor order.
// TODO(not planned): future row selection precedes the public folds so an
// owner-specific selection can recompute totals. No expression engine is added.
//
//go:norace
func (k *Kernel) SyscallStats(ref contracts.MachineRef) ([]contracts.SyscallRecord, error) {
	machine, err := k.Lookup(ref)
	if err != nil {
		return nil, err
	}
	var rows []contracts.SyscallRecord
	for slot, counter := range machine.syscalls.values {
		if counter.total == 0 {
			continue
		}
		info := syscallInfo(RouteID(slot))
		for group, count := range counter.byDescriptor {
			if count == 0 {
				continue
			}
			rows = append(rows, contracts.SyscallRecord{
				Name: info.Name, Group: info.Group, DescriptorBased: info.DescriptorBased,
				DescriptorGroup: descriptorGroupName(contracts.DescriptorGroup(group)), Count: count,
			})
		}
	}
	return rows, nil
}

// TODO(not planned): catalogue selection can use only static metadata; observed
// descriptor groups and counts do not exist for uncalled entries.
//
//go:norace
func (k *Kernel) CountableSyscalls(ref contracts.MachineRef) ([]contracts.SyscallInfo, error) {
	if _, err := k.Lookup(ref); err != nil {
		return nil, err
	}
	var result []contracts.SyscallInfo
	for slot, metadata := range syscallCatalogue {
		if metadata.name != "" && k.features.Has(metadata.group) {
			result = append(result, syscallInfo(RouteID(slot)))
		}
	}
	return result, nil
}
