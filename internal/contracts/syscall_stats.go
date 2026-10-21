package contracts

// DescriptorGroup classifies an input descriptor at syscall admission.
type DescriptorGroup uint8

const (
	DescriptorNone DescriptorGroup = iota
	DescriptorRT
	DescriptorFS
	DescriptorNet
	DescriptorUnknown
	DescriptorGroupLimit
)

// SyscallRecord is an owned snapshot row for one name and descriptor group.
// Count is the bucket count, not the combined count across descriptor groups.
type SyscallRecord struct {
	Name            string
	Group           string
	DescriptorBased bool
	DescriptorGroup string
	Count           uint64
}

// SyscallInfo describes a countable syscall independently of observed activity.
type SyscallInfo struct {
	Name            string
	Group           string
	DescriptorBased bool
}
