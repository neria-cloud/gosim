package abi

// IDs carry no translated type identity across the native boundary. Run IDs
// guard handle lifetime and are excluded from replay and random derivation.
type (
	RunID     uint64
	MachineID uint64
	EpochID   uint64
	CallID    uint64
)

type Diagnostic struct {
	Kind, Feature, Operation, Message string
	RunID, MachineID, Epoch           uint64
	PC                                uintptr
}

type HostIdentity struct {
	Toolchain, Profile, HostOS, HostArch, SourceDigest string
}
