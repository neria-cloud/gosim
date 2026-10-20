// Package buildspec defines host-side values shared by build configuration and
// compatibility profiles. It has no dependency on executable Go adapters.
package buildspec

type Config struct {
	Schema       int      `json:"schema"`
	Features     []string `json:"features"`
	SimulationOS string   `json:"simulation_os"`
}

type Identity struct {
	Schema            int        `json:"schema"`
	Toolchain         string     `json:"toolchain"`
	ToolchainDigest   string     `json:"toolchain_digest"`
	StdlibDigest      string     `json:"stdlib_digest"`
	Profile           string     `json:"profile"`
	AdapterRevision   string     `json:"adapter_revision"`
	ContractDigest    string     `json:"contract_digest"`
	RuntimeRevision   string     `json:"runtime_revision"`
	ProviderRevisions []Revision `json:"provider_revisions"`
	HostOS            string     `json:"host_os"`
	HostArch          string     `json:"host_arch"`
	SimulationOS      string     `json:"simulation_os"`
	SimulationArch    string     `json:"simulation_arch"`
	Features          []string   `json:"features"`
	Race              bool       `json:"race"`
	Experiments       string     `json:"experiments"`
	Tags              []string   `json:"tags"`
	ModuleGraphDigest string     `json:"module_graph_digest"`
	TranslatorDigest  string     `json:"translator_digest"`
	DebugPolicyDigest string     `json:"debug_policy_digest"`
}

type Revision struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type CertificationTuple struct {
	Toolchain      string   `json:"toolchain"`
	HostOS         string   `json:"host_os"`
	HostArch       string   `json:"host_arch"`
	SimulationOS   string   `json:"simulation_os"`
	SimulationArch string   `json:"simulation_arch"`
	Race           bool     `json:"race"`
	Features       []string `json:"features"`
	Experiments    string   `json:"experiments"`
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
	// ModuleCache is the location resolved by Go before GOENV is disabled.
	// It is transient build context, not part of executable identity.
	ModuleCache string `json:"-"`
}

type BuildOptions struct {
	Race        bool
	Tags        []string
	Experiments string
}

// Fingerprints identify installed inputs, not an allowlist of distribution archives.
type Fingerprints struct {
	ToolchainDigest string `json:"toolchain_digest"`
	StdlibDigest    string `json:"stdlib_digest"`
}

type ModuleContext struct {
	Directory   string `json:"directory"`
	GoMod       string `json:"go_mod"`
	GoWork      string `json:"go_work"`
	GraphDigest string `json:"graph_digest"`
}

type Resolution struct {
	Schema    int           `json:"schema"`
	Toolchain Toolchain     `json:"toolchain"`
	Context   ModuleContext `json:"context"`
}
