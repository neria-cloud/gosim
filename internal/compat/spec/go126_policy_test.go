package spec_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
)

func TestGo126PolicyDeltas(t *testing.T) {
	t.Parallel()
	p, m := p126.Descriptor(), p126.PolicyManifest()
	sources := make(map[string]bool)
	for _, source := range fixtureOrigin(t, "go126").Files {
		sources[source.Path] = true
	}
	for _, probe := range m.Probes {
		if !sources[probe.Source] {
			t.Fatalf("unverified probe source: %+v", probe)
		}
	}
	for _, path := range []string{"testing/cryptotest", "crypto/internal/rand", "crypto/internal/entropy/v1.0.0", "crypto/internal/fips140deps/time", "internal/runtime/syscall/linux", "internal/runtime/gc/scan", "internal/runtime/pprof/label"} {
		if _, err := p.Policy(path, true); err == nil || !strings.Contains(err.Error(), "unsupported application") {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if policy, err := p.Policy("internal/strconv", true); err != nil || policy.Mode != "translate" {
		t.Fatalf("integer helpers: %+v %v", policy, err)
	}
	if probe, err := m.Probe("syscall", "runtimeClearenv", []string{"rt"}, nil); err != nil || probe.Feature != "rt" {
		t.Fatalf("clearenv: %+v %v", probe, err)
	}
	if _, err := m.Probe("testing", "*common.ArtifactDir", []string{"rt"}, nil); err == nil || !strings.Contains(err.Error(), "FeatureDisabled") {
		t.Fatalf("artifact gate: %v", err)
	}
	if _, err := m.Probe("testing/cryptotest", "SetGlobalRandom", []string{"rt", "fs", "net"}, nil); err == nil {
		t.Fatal("native global override accepted")
	}
	// Native imports remain exact owner/package records, never prefix admission.
	for _, entry := range m.NativeImports {
		if _, err := m.ResolveSymbol(p, "runtime", "newUnreviewedSymbol", true); err == nil {
			t.Fatal("unknown ABI admitted")
		}
		if err := m.AllowNativeImport(p, entry.From+"/unreviewed", entry.Package, true); err == nil {
			t.Fatal("native import permission escaped owner")
		}
	}
	if strings.HasPrefix(runtime.Version(), "go1.26.") {
		for _, probe := range m.Probes {
			data, err := os.ReadFile(filepath.Join(runtime.GOROOT(), probe.Source))
			if err != nil {
				t.Fatal(err)
			}
			symbol := probe.Symbol
			if at := strings.LastIndexByte(symbol, '.'); at >= 0 {
				symbol = symbol[at+1:]
			}
			if !strings.Contains(string(data), symbol) {
				t.Fatalf("probe source no longer names %s", probe.Symbol)
			}
		}
		if err := p126.VerifyModules(currentToolchain(), "../../.."); err != nil {
			t.Fatal(err)
		}
	}
}
