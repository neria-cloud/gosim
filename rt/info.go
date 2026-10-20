package rt

type RunInfo struct {
	Seed                                                 int64
	Toolchain, Profile, AdapterRevision, RuntimeRevision string
	Features                                             []string
	HostOS, HostArch, SimulationOS, SimulationArch       string
	BuildDigest, ReplayRevision                          string
	// Deprecated: certification is an external report, not runtime state. Always false.
	Certified bool
}
