// Package contracts defines translated domain contracts shared by the kernel
// and optional providers. Its only Gosim dependency is the native memory ABI.
package contracts

import "fmt"

type (
	Feature    uint8 // RT=1, FS=2, Net=4
	FeatureSet uint8
)

type (
	MachineRef struct{ RunID, ID uint64 }
	EpochRef   struct {
		Machine MachineRef
		Epoch   uint64
	}
)

type CallContext struct {
	Epoch                     EpochRef
	InvocationID, GoroutineID uint64
	PC                        uintptr
}
type (
	StopKind    uint8 // StopGraceful, StopCrash
	CloseReason uint8 // CloseExplicit, CloseGraceful, CloseCrash, CloseAbort
	EventID     uint64
	WaitID      uint64
	Readiness   uint8 // ReadReady, WriteReady, Hangup, ErrorReady bitset
	Diagnostic  struct {
		Kind, Feature, Operation, Message string
		Epoch                             EpochRef
		PC                                uintptr
	}
)

const (
	RT Feature = 1 << iota
	FS
	Net
)

const (
	StopGraceful StopKind = iota
	StopCrash
)

const (
	CloseExplicit CloseReason = iota
	CloseGraceful
	CloseCrash
	CloseAbort
)

const (
	ReadReady Readiness = 1 << iota
	WriteReady
	Hangup
	ErrorReady
)

func (set FeatureSet) Has(feature Feature) bool {
	return (feature == RT || feature == FS || feature == Net) && set&FeatureSet(feature) != 0
}

func ParseFeatures(names []string) (FeatureSet, error) {
	var set FeatureSet
	for _, name := range names {
		var feature Feature
		switch name {
		case "rt":
			feature = RT
		case "fs":
			feature = FS
		case "net":
			feature = Net
		default:
			return 0, fmt.Errorf("%w: unknown feature %q", ErrInvalidConfig, name)
		}
		if set.Has(feature) {
			return 0, fmt.Errorf("%w: duplicate feature %q", ErrInvalidConfig, name)
		}
		set |= FeatureSet(feature)
	}
	return set, set.Validate()
}

func (set FeatureSet) Validate() error {
	// TODO(not planned): fs/net with native scheduling or mutable feature sets.
	if !set.Has(RT) || set&^FeatureSet(RT|FS|Net) != 0 {
		return fmt.Errorf("%w: feature set must include rt and only rt/fs/net", ErrInvalidConfig)
	}
	return nil
}

func (set FeatureSet) Names() []string {
	var names []string
	for _, item := range []struct {
		feature Feature
		name    string
	}{{RT, "rt"}, {FS, "fs"}, {Net, "net"}} {
		if set.Has(item.feature) {
			names = append(names, item.name)
		}
	}
	return names
}
