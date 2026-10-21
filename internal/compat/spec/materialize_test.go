package spec

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/testutil"
)

func resolvedFixture() ResolvedFamily {
	p, _ := fixture()
	r := ResolvedFamily{
		Family: "1.26", ProfileID: "go126", Supported: true, Chain: []string{"1.25", "1.26"}, DefinitionDigest: strings.Repeat("d", 64),
		Packages: p.Packages, RuntimePackages: []PackageRef{{Path: module + "/rt/runtime"}}, Architectures: map[string]ArchitectureRecords{},
		Bridges: []Bridge{{Package: "runtime", NativeSymbols: []string{"Gosched"}, Reason: "runtime boundary"}},
		Probes:  []InitProbe{{Package: "runtime", Symbol: "probe", Phase: "init", Disposition: "modeled", Feature: "rt", Operation: "probe", Response: "ok", Source: "src/runtime/x.go", Arguments: map[string][]int64{"x": {1, 2}}}},
	}
	hook := p.Hooks[0]
	hook.Target = strings.Replace(hook.Target, "/go126/", "/", 1)
	r.Declarations = []Declaration{{Hook: hook}}
	for _, arch := range []string{"amd64", "arm64"} {
		m := p.ContractsByArch[arch]
		r.Architectures[arch] = ArchitectureRecords{Layouts: m.Layouts, Constants: m.Constants}
		variant := hook
		variant.Symbol = "archHook"
		variant.Signature = "func(" + map[string]string{"amd64": "int64", "arm64": "uint64"}[arch] + ")"
		r.Declarations = append(r.Declarations, Declaration{Hook: variant, Arches: []string{arch}})
	}
	for _, binding := range []struct{ pkg, symbol, signature, shim, owner string }{
		{"crypto/tls", "CipherSuites", "func() []*CipherSuite", "tls", "net"},
		{"crypto/tls", "InsecureCipherSuites", "func() []*CipherSuite", "tls", "net"},
		{"runtime/metrics", "All", "func() []Description", "metrics", "rt"},
		{"runtime/metrics", "Read", "func([]Sample)", "metrics", "rt"},
		{"runtime/metrics", "Value.Kind", "func() ValueKind", "metrics", "rt"},
		{"runtime/metrics", "Value.Uint64", "func() uint64", "metrics", "rt"},
		{"runtime/metrics", "Value.Float64", "func() float64", "metrics", "rt"},
		{"runtime/metrics", "Value.Float64Histogram", "func() *Float64Histogram", "metrics", "rt"},
		{"runtime/debug", "ReadGCStats", "func(*GCStats)", "debug", "rt"},
	} {
		r.Declarations = append(r.Declarations, Declaration{Hook: Hook{Package: binding.pkg, Symbol: binding.symbol, Signature: binding.signature, Owner: binding.owner, Target: module + "/internal/compat/shims/" + binding.shim + "." + binding.symbol}})
	}
	for _, context := range []string{"host", "host-race"} {
		r.Declarations = append(r.Declarations, Declaration{Hook: Hook{Package: "runtime", Symbol: context, Signature: "func()", Owner: "rt", Context: context, Target: module + "/internal/compat/host.F"}})
	}
	return r
}

func TestMaterializeBothArchitectureContracts(t *testing.T) {
	r := resolvedFixture()
	for _, arch := range []string{"amd64", "arm64"} {
		p, err := r.Descriptor(arch, strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(p.Hooks) != 2 || len(p.ContractsByArch) != 2 {
			t.Fatal("wrong callable/contracts views")
		}
		shims := r.ShimManifest(arch)
		if len(shims) != 9 {
			t.Fatal("nine omitted legacy bindings not materialized")
		}
		for _, contractArch := range []string{"amd64", "arm64"} {
			manifest, err := p.Contracts(contractArch)
			if err != nil || len(manifest.Hooks) != 13 {
				t.Fatal("wrong complete contract manifest", err)
			}
			for _, shim := range shims {
				if !slices.Contains(manifest.Hooks, shim) {
					t.Fatalf("missing %s.%s in %s", shim.Package, shim.Symbol, contractArch)
				}
			}
			for _, hook := range manifest.Hooks {
				if hook.Symbol == "archHook" {
					want := "func(int64)"
					if contractArch == "arm64" {
						want = "func(uint64)"
					}
					if hook.Signature != want {
						t.Fatal("cross-architecture contract leakage")
					}
				}
			}
		}
		if p.DefinitionDigest != r.DefinitionDigest || !reflect.DeepEqual(p.Chain, r.Chain) {
			t.Fatal("resolution identity lost")
		}
		p.Chain[0] = "mutated"
		p.ContractsByArch["arm64"].Layouts[0].Fields[0].Name = "mutated"
		if r.Chain[0] != "1.25" || r.Architectures["arm64"].Layouts[0].Fields[0].Name == "mutated" {
			t.Fatal("descriptor retains mutable input")
		}
	}
	policy := r.PolicyManifest()
	if err := policy.Validate(func() Profile { p, _ := r.Descriptor("arm64", strings.Repeat("a", 64)); return p }()); err != nil {
		t.Fatal(err)
	}
	policy.Probes[0].Arguments["x"][0] = 99
	policy.Bridges[0].NativeSymbols[0] = "changed"
	if r.Probes[0].Arguments["x"][0] != 1 || r.Bridges[0].NativeSymbols[0] != "Gosched" {
		t.Fatal("policy retains mutable input")
	}
}

func TestPureMaterializationKeepsProductionAdmissionStrict(t *testing.T) {
	r := resolvedFixture()
	r.Family = testutil.UnregisteredGo().Family
	r.ProfileID = testutil.UnregisteredGo().Profile
	for minor := 25 + len(r.Chain); minor <= testutil.UnregisteredGo().Minor; minor++ {
		r.Chain = append(r.Chain, fmt.Sprintf("1.%d", minor))
	}
	r.Supported = false
	p, err := r.Descriptor("arm64", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal("unregistered pure view rejected", err)
	}
	if p.Validate() == nil {
		t.Fatal("unregistered production profile accepted")
	}
	for _, arch := range []string{"", "mips"} {
		if _, err := r.Descriptor(arch, strings.Repeat("a", 64)); err == nil {
			t.Fatal("invalid architecture accepted")
		}
	}
}

func TestMaterializationRejectsInvalidDeclarations(t *testing.T) {
	for _, change := range []func(*Hook){
		func(h *Hook) { h.Context = "other" },
		func(h *Hook) { h.Context = "host" },
		func(h *Hook) { h.Target = module + "/internal/compat/host.F" },
		func(h *Hook) { h.Target = module + "/internal/compat/profiles.F" },
		func(h *Hook) { h.Target = module + "/internal/compat/hooks/.F" },
		func(h *Hook) { h.Target = module + "/internal/compat/hooks/rt/../bad.F" },
	} {
		r := resolvedFixture()
		change(&r.Declarations[0].Hook)
		if _, err := r.Descriptor("amd64", strings.Repeat("a", 64)); err == nil {
			t.Fatalf("invalid declaration accepted: %+v", r.Declarations[0])
		}
	}
	r := resolvedFixture()
	r.Architectures["amd64"] = ArchitectureRecords{}
	if _, err := r.Descriptor("arm64", strings.Repeat("a", 64)); err == nil {
		t.Fatal("missing other architecture contracts accepted")
	}
}

func TestContractDigestUsesResolvedContractsOnly(t *testing.T) {
	r := resolvedFixture()
	p, err := r.Descriptor("arm64", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := p.ContractDigest()
	p.DefinitionDigest = strings.Repeat("b", 64)
	same, _ := p.ContractDigest()
	if before != same {
		t.Fatal("diagnostic ancestry changed contract hash schema")
	}
	m := p.ContractsByArch["amd64"]
	m.Constants[0].Value = "2"
	p.ContractsByArch["amd64"] = m
	after, _ := p.ContractDigest()
	if before == after {
		t.Fatal("inactive callable architecture absent from contract identity")
	}
}
