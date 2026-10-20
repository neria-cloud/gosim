package fs

type CrashMode uint8

const (
	DurableOnly CrashMode = iota
	SamplePending
)

type (
	SyncFault    struct{ Probability, BeforeProbability float64 }
	StorageStats struct {
		TotalBytes uint64
		FreeBytes  uint64
		UsedBytes  uint64
		BlockSize  uint64
		ReadOnly   bool
	}
	InodeInfo struct {
		MemLinks     int
		MemExists    bool
		DiskLinks    int
		DiskExists   bool
		Handles, Ops int
	}
)
