package selected

import (
	"slices"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestResolveArchitectureAndOwnedMetadata(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			p, err := descriptorForArch(arch)
			if err != nil {
				t.Fatal(err)
			}
			patch := "go1.26.9"
			if p.ID == "go125" {
				patch = "go1.25.14"
			}
			d := strings.Repeat("a", 64)
			id := buildspec.Identity{
				Schema: 1, Toolchain: patch, Profile: p.ID, AdapterRevision: p.Revision,
				DistributionDigest: d, StdlibDigest: d, RuntimeRevision: d, HostOS: "linux", HostArch: arch, SimulationOS: "linux", SimulationArch: arch,
				Features: []string{"rt"}, Tags: []string{"linkname", "sim"}, ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d, Certification: "candidate",
			}
			s, err := p.Select(id)
			if err != nil {
				t.Fatal(err)
			}
			resolved, manifest, err := Resolve(s)
			if err != nil {
				t.Fatal(err)
			}
			haveCPU := false
			for _, hook := range resolved.Hooks {
				if hook.Package == "internal/cpu" && hook.Symbol == "cpuid" {
					haveCPU = true
				}
			}
			if haveCPU != (arch == "amd64") {
				t.Fatal("architecture-specific hooks were not selected")
			}
			resolved.Hooks[0].Target = "mutated"
			resolved.RuntimePackages[0] = "mutated"
			manifest.Bridges[0].Package = "mutated"
			again, againManifest, err := Resolve(s)
			if err != nil {
				t.Fatal(err)
			}
			if again.Hooks[0].Target == "mutated" || againManifest.Bridges[0].Package == "mutated" || !slices.Equal(again.RuntimePackages, p.RuntimePackages) {
				t.Fatal("metadata ownership leaked")
			}
			other := arch
			if arch == "arm64" {
				other = "amd64"
			} else {
				other = "arm64"
			}
			s.Identity.HostArch = other
			s.Identity.SimulationArch = other
			if _, _, err := Resolve(s); err == nil {
				t.Fatal("other architecture's hook digest accepted")
			}
		})
	}
}
