package spec_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestRuntimeFamilyABIDelta(t *testing.T) {
	t.Parallel()
	for _, item := range []struct {
		p      spec.Profile
		result string
	}{
		{p125.Descriptor(), " uintptr"}, {p126.Descriptor(), ""},
	} {
		found := false
		for _, hook := range item.p.Hooks {
			if hook.Owner != "rt" {
				continue
			}
			if !strings.Contains(hook.Target, "/"+item.p.ID+"/") {
				t.Fatalf("cross-family target: %+v", hook)
			}
			if hook.Package == "sync" && hook.Symbol == "runtime_StoreReluintptr" {
				found = true
				if hook.Signature != "func(*uintptr, uintptr)"+item.result {
					t.Fatalf("unrecorded store ABI: %+v", hook)
				}
				if strings.HasPrefix(runtime.Version(), strings.Replace(item.p.ID, "go1", "go1.", 1)+".") {
					mutated := hook
					mutated.Signature += "bool"
					if err := spec.VerifyHooks(currentToolchain(), []spec.Hook{mutated}); err == nil {
						t.Fatal("store result mutation accepted")
					}
					mutated = hook
					mutated.Symbol = "runtime_missingRequiredHook"
					if err := spec.VerifyHooks(currentToolchain(), []spec.Hook{mutated}); err == nil {
						t.Fatal("missing runtime hook accepted")
					}
				}
			}
		}
		if !found {
			t.Fatalf("%s missing store hook", item.p.ID)
		}
	}
}

func TestGo126RuntimePolicy(t *testing.T) {
	t.Parallel()
	p, m := p126.Descriptor(), p126.PolicyManifest()
	if err := m.Validate(p); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"Acquire", "ReadObjectPC", "WriteObjectPC", "ReadRange", "WriteRange"} {
		target, err := m.ResolveSymbol(p, "internal/race", symbol, true)
		if err != nil || !strings.Contains(target.Package, "/go126/shims/runtimebridge") {
			t.Fatalf("race %s: %+v %v", symbol, target, err)
		}
	}
	if _, err := m.ResolveSymbol(p, "internal/race", "missingRequiredHook", true); err == nil {
		t.Fatal("unknown race symbol accepted")
	}
	for _, path := range []string{"time", "sync", "maps", "math/rand/v2"} {
		policy, err := p.Policy(path, true)
		if err != nil || policy.Mode != "translate" {
			t.Fatalf("%s: %+v %v", path, policy, err)
		}
	}
	if _, err := p.Policy("future/stdlib", true); err == nil {
		t.Fatal("unknown stdlib accepted")
	}
}

func TestPinnedRuntimePushesObjectRaceSymbols(t *testing.T) {
	t.Parallel()
	if runtime.Version() != "go1.25.14" && runtime.Version() != "go1.26.9" {
		t.Skip("pinned source audit")
	}
	// Both pinned families push these symbols. A declaration-only shim cannot
	// replace their implementation without redirecting the translated call sites.
	source, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src/runtime/race.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"ReadObjectPC", "WriteObjectPC"} {
		if !strings.Contains(string(source), "//go:linkname race_"+symbol+" internal/race."+symbol) {
			t.Fatalf("missing runtime push for %s", symbol)
		}
	}
}
