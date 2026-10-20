//go:build sim

package fs

import (
	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/control"
	"github.com/neria-cloud/gosim/internal/kernel"
)

func SetCrashMode(m gosim.Machine, mode CrashMode) error {
	_, err := control.Call(kernel.FSSetCrashModeRequest{Machine: ref(m), Mode: uint8(mode)}, kernel.DecodeFSSetCrashModeResult)
	return err
}

func SetSyncFault(m gosim.Machine, fault SyncFault) error {
	_, err := control.Call(kernel.FSSetSyncFaultRequest{Machine: ref(m), Fault: contracts.FSFaultConfig(fault)}, kernel.DecodeFSSetSyncFaultResult)
	return err
}

func SetReadOnly(m gosim.Machine, enabled bool) error {
	_, err := control.Call(kernel.FSSetReadOnlyRequest{Machine: ref(m), Enabled: enabled}, kernel.DecodeFSSetReadOnlyResult)
	return err
}

func Stats(m gosim.Machine) (StorageStats, error) {
	r, err := control.Call(kernel.FSStatsRequest{Machine: ref(m)}, kernel.DecodeFSStatsResult)
	return StorageStats(r.Stats), err
}

func InspectInode(m gosim.Machine, inode int) (InodeInfo, error) {
	r, err := control.Call(kernel.FSInspectInodeRequest{Machine: ref(m), Inode: inode}, kernel.DecodeFSInspectInodeResult)
	return InodeInfo(r.Info), err
}
