package fs

type CrashMode uint8

const (
	DurableOnly CrashMode = iota
	SamplePending
)

type (
	SyncFault struct{ Probability, BeforeProbability float64 }
	InodeInfo struct {
		MemLinks     int
		MemExists    bool
		DiskLinks    int
		DiskExists   bool
		Handles, Ops int
	}
)
