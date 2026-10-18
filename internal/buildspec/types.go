// Package buildspec defines host-side values shared by build configuration and
// compatibility profiles. It has no dependency on executable Go adapters.
package buildspec

type Config struct {
	Schema       int      `json:"schema"`
	Toolchain    string   `json:"toolchain"`
	Features     []string `json:"features"`
	SimulationOS string   `json:"simulation_os"`
}

type Identity struct {
	Schema             int        `json:"schema"`
	Toolchain          string     `json:"toolchain"`
	DistributionDigest string     `json:"distribution_digest"`
	StdlibDigest       string     `json:"stdlib_digest"`
	Profile            string     `json:"profile"`
	AdapterRevision    string     `json:"adapter_revision"`
	RuntimeRevision    string     `json:"runtime_revision"`
	ProviderRevisions  []Revision `json:"provider_revisions"`
	HostOS             string     `json:"host_os"`
	HostArch           string     `json:"host_arch"`
	SimulationOS       string     `json:"simulation_os"`
	SimulationArch     string     `json:"simulation_arch"`
	Features           []string   `json:"features"`
	Race               bool       `json:"race"`
	Experiments        string     `json:"experiments"`
	Tags               []string   `json:"tags"`
	ModuleGraphDigest  string     `json:"module_graph_digest"`
	TranslatorDigest   string     `json:"translator_digest"`
	DebugPolicyDigest  string     `json:"debug_policy_digest"`
	Certification      string     `json:"certification"`
	EvidenceDigest     string     `json:"evidence_digest"`
}

type Revision struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type Tuple struct {
	Toolchain      string   `json:"toolchain"`
	HostOS         string   `json:"host_os"`
	HostArch       string   `json:"host_arch"`
	SimulationOS   string   `json:"simulation_os"`
	SimulationArch string   `json:"simulation_arch"`
	Race           bool     `json:"race"`
	Features       []string `json:"features"`
	Experiments    string   `json:"experiments"`
	Status         string   `json:"status"`
	SourceDigest   string   `json:"source_digest"`
	ResultDigest   string   `json:"result_digest"`
}

type Selection struct {
	Identity           Identity
	HookManifestDigest string
	RuntimePackages    []string
	ProviderImports    []string
}

// These records are transient; persisted artifacts use Identity instead.
type Toolchain struct {
	Executable, Version, GOROOT, HostOS, HostArch string
}

type BuildOptions struct {
	Race        bool
	Tags        []string
	Experiments string
}
