//go:build !sim

package fs

import "github.com/neria-cloud/gosim"

func SetCrashMode(gosim.Machine, CrashMode) error        { return gosim.ErrNotSim }
func SetSyncFault(gosim.Machine, SyncFault) error        { return gosim.ErrNotSim }
func InspectInode(gosim.Machine, int) (InodeInfo, error) { return InodeInfo{}, gosim.ErrNotSim }
