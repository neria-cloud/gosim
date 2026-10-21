package spec

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestObservedAPIScopeAndMethodSets(t *testing.T) {
	r := ResolvedFamily{Declarations: []Declaration{{Hook: Hook{Package: "runtime", Context: "host"}}, {Hook: Hook{Package: "reflect"}}, {Hook: Hook{Package: module + "/rt/runtime"}}}, Bridges: []Bridge{{Package: "crypto/tls", Replacement: "shim"}, {Package: "runtime/metrics", NativeSymbols: []string{"Read"}}, {Package: module + "/internal/kernel"}}, Packages: []PackagePolicy{{Path: "net/..."}}}
	if got := ObservationAPIPackages(r); !reflect.DeepEqual(got, []string{"crypto/tls", "reflect", "runtime", "runtime/metrics"}) {
		t.Fatal(got)
	}
	// A whole-package bridge with no declarations still contributes every API.
	r.Declarations = nil
	r.Bridges = []Bridge{{Package: "fixture", Replacement: "shim"}}
	dir := t.TempDir()
	write := func(name, code string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(code), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module fixture\ngo 1.25\n")
	write("api.go", "package fixture\ntype Record struct{ X int };func (Record) Value()int{return 0};func (*Record) Pointer(){};func Exported(x int){};func hidden(){};const Limit=42\n")
	root, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		t.Fatal(err)
	}
	tc := buildspec.Toolchain{Version: runtime.Version(), GOROOT: root, Executable: filepath.Join(root, "bin/go"), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	req := ObservationRequest{Family: r, Contexts: []SourceContext{{OS: "linux", Arch: runtime.GOARCH}}, IncludeAPIs: true}
	collect := func() ObservationSet {
		t.Helper()
		out, err := CollectSourceObservations(t.Context(), tc, dir, req)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := collect()
	if !reflect.DeepEqual(before, collect()) {
		t.Fatal("API results not deterministic")
	}
	api := func(set ObservationSet) map[string]string {
		m := map[string]string{}
		for _, record := range set.Records {
			if record.Kind == "api" {
				m[record.Key[1]] = string(record.Value)
			}
		}
		return m
	}
	old := api(before)
	for _, name := range []string{"Record", "Record.Value", "*Record.Value", "*Record.Pointer", "Exported", "Limit"} {
		if old[name] == "" {
			t.Fatal("missing API", name, old)
		}
	}
	if old["hidden"] != "" || old["Record.Pointer"] != "" {
		t.Fatal("wrong exported method set", old)
	}
	write("api.go", "package fixture\ntype Record struct{ X string };func (*Record) Pointer(x int){};func Added(){};const Limit=43\n")
	after := api(collect())
	if after["Added"] == "" || after["Exported"] != "" || after["Record.Value"] != "" || after["*Record.Pointer"] == old["*Record.Pointer"] || after["Limit"] == old["Limit"] {
		t.Fatal(old, after)
	}
	req.IncludeAPIs = false
	skipped := collect()
	if len(skipped.APIPackages) != 0 || len(api(skipped)) != 0 {
		t.Fatal("declaration-only scan collected APIs")
	}
	req.IncludeAPIs = true
	r.Bridges[0].Package = "internal/gosim_missing_api_package"
	req.Family = r
	failed, err := CollectSourceObservations(context.Background(), tc, dir, req)
	if err != nil || !failed.Contexts[0].Complete || len(api(failed)) != 0 {
		t.Fatal("absent API package was not represented as a complete empty inventory", failed, err)
	}
}

func TestActualParentAPIsIncludeWholePackageBridges(t *testing.T) {
	t.Parallel()
	defs, err := LoadDefinitions(os.DirFS("../profiles"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := buildspec.ParseGoVersion(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := ResolveDefinitions(defs, v.Family())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		t.Fatal(err)
	}
	tc := buildspec.Toolchain{Version: runtime.Version(), GOROOT: root, Executable: filepath.Join(root, "bin/go"), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	dir, cleanup, err := PrepareSourceInspectionContext(t.Context(), buildspec.Resolution{Toolchain: tc})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	out, err := CollectSourceObservations(t.Context(), tc, dir, ObservationRequest{Family: parent, Contexts: []SourceContext{{OS: "linux", Arch: runtime.GOARCH}, {OS: runtime.GOOS, Arch: runtime.GOARCH, Native: true}, {OS: runtime.GOOS, Arch: runtime.GOARCH, Native: true, Race: true}}, IncludeAPIs: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"crypto/tls", "crypto/x509", "runtime/metrics", "runtime/debug"} {
		if !slices.Contains(out.APIPackages, path) {
			t.Fatal("bridge scope omitted", path)
		}
	}
	found := map[string]bool{}
	for _, r := range out.Records {
		if r.Kind == "api" {
			found[r.Key[0]+"."+r.Key[1]] = json.Valid(r.Value)
		}
	}
	for _, key := range []string{"crypto/tls.Config", "crypto/x509.Certificate", "runtime/metrics.Description", "runtime/debug.BuildInfo"} {
		if !found[key] {
			t.Fatal("whole-package API missing", key)
		}
	}
}
