package spec_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/testutil"
)

func TestSourceObservationsThroughPrivateModule(t *testing.T) {
	root := t.TempDir()
	mod := []byte("module example.org/observation\ngo 1.25\n")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), mod, 0o600); err != nil {
		t.Fatal(err)
	}
	write := func(code string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "source.go"), []byte(code), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	base := "package observation\ntype Record struct { X int64 }; const Limit = 42\nfunc Hook(x int) int {return x}\n"
	write(base)
	c, err := buildcfg.DiscoverInspection(t.Context(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	dir, cleanup, err := spec.PrepareSourceInspectionContext(t.Context(), buildspec.Resolution{Toolchain: c.Toolchain(), Context: buildspec.ModuleContext{Directory: root, GoWork: "off"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	pkg := "example.org/observation"
	req := spec.ObservationRequest{Family: spec.ResolvedFamily{
		Declarations:  []spec.Declaration{{Hook: spec.Hook{Package: pkg, Symbol: "Hook"}}},
		Architectures: map[string]spec.ArchitectureRecords{runtime.GOARCH: {Layouts: []spec.LayoutContract{{Package: pkg, Type: "Record"}}, Constants: []spec.ConstantContract{{Package: pkg, Symbol: "Limit"}}}},
	}, Contexts: []spec.SourceContext{{OS: "linux", Arch: runtime.GOARCH}}, Origin: spec.Origin{Files: []spec.SourceFile{{Path: "src/gosim_missing_file.go"}}}}
	collect := func() spec.ObservationSet {
		t.Helper()
		got, err := spec.CollectSourceObservations(t.Context(), c.Toolchain(), dir, req)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Contexts) != 1 || !got.Contexts[0].Complete {
			t.Fatal(got)
		}
		return got
	}
	before := collect()
	if !reflect.DeepEqual(before, collect()) {
		t.Fatal("nondeterministic observation")
	}
	get := func(set spec.ObservationSet, kind string) spec.Observation {
		t.Helper()
		for _, r := range set.Records {
			if r.Kind == kind && (kind != "source" || r.Present) {
				return r
			}
		}
		t.Fatal("missing record", kind)
		return spec.Observation{}
	}
	if string(get(before, "declaration").Value) != `"func(int) int"` || string(get(before, "constant").Value) != `"42"` {
		t.Fatal(before)
	}
	var layout spec.LayoutContract
	if err := json.Unmarshal(get(before, "layout").Value, &layout); err != nil || layout.Size != 8 {
		t.Fatal(layout, err)
	}
	for _, change := range []struct {
		code, kind string
		missing    bool
	}{
		{"package observation\ntype Record struct { X int64 }; const Limit=42; func Hook(x string) int{return 0}", "declaration", false},
		{"package observation\ntype Record struct { X int64 }; const Limit=42", "declaration", true},
		{"package observation\ntype Record struct { X [2]int64 }; const Limit=42; func Hook(x int) int{return x}", "layout", false},
		{"package observation\nconst Limit=42; func Hook(x int) int{return x}", "layout", true},
		{"package observation\ntype Record struct { X int64 }; const Limit=43; func Hook(x int) int{return x}", "constant", false},
		{"package observation\ntype Record struct { X int64 }; func Hook(x int) int{return x}", "constant", true},
	} {
		write(change.code)
		after := collect()
		got := get(after, change.kind)
		if got.Present == change.missing || reflect.DeepEqual(got.Value, get(before, change.kind).Value) {
			t.Fatal("change not observed", change, got)
		}
		if reflect.DeepEqual(get(after, "source").Value, get(before, "source").Value) {
			t.Fatal("source change not hashed")
		}
	}
	write("package observation\nfunc broken(")
	failed, err := spec.CollectSourceObservations(t.Context(), c.Toolchain(), dir, req)
	if err == nil || len(failed.Records) != 0 || failed.Contexts[0].Complete {
		t.Fatal("failed parse fabricated absence", failed, err)
	}
	write(base)
	req.Family.Declarations[0].Hook.Package = "internal/gosim_missing_package"
	failed, err = spec.CollectSourceObservations(t.Context(), c.Toolchain(), dir, req)
	if err != nil || !failed.Contexts[0].Complete {
		t.Fatal("absent standard package aborted inspection", err)
	}
	if got := get(failed, "declaration"); got.Present {
		t.Fatal("missing package declaration present")
	}
	req.Family.Declarations[0].Hook.Package = pkg
	write("package observation\nimport _ \"example.org/observation/missing\"\n")
	failed, err = spec.CollectSourceObservations(t.Context(), c.Toolchain(), dir, req)
	if err == nil || failed.Contexts[0].Complete {
		t.Fatal("load failure accepted")
	}
	write(base)
	req.Family.Declarations[0].Hook.Package = pkg
	req.IncludeStdlibInventory = true
	inventory := collect()
	found := false
	for _, r := range inventory.Records {
		if r.Kind == "package" && r.Key[0] == "net/http" {
			found = true
		}
	}
	if !found {
		t.Fatal("stdlib inventory missing")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := spec.CollectSourceObservations(cancelled, c.Toolchain(), dir, req); err == nil {
		t.Fatal("ignored cancellation")
	}
	wrong := c.Toolchain()
	wrong.Version = testutil.UnregisteredGo().Version
	if _, err := spec.CollectSourceObservations(t.Context(), wrong, dir, req); err == nil {
		t.Fatal("accepted other compiler")
	}
	if err := os.WriteFile(filepath.Join(root, "cgo.go"), []byte("//go:build cgo\n\npackage observation\nimport \"C\"\nfunc CgoOnly(){}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	req.IncludeStdlibInventory = false
	req.Family.Declarations = []spec.Declaration{{Hook: spec.Hook{Package: pkg, Symbol: "CgoOnly", Context: "host"}}}
	for _, race := range []bool{false, true} {
		req.Contexts = []spec.SourceContext{{OS: runtime.GOOS, Arch: runtime.GOARCH, Native: true, Race: race}}
		if got := get(collect(), "declaration"); got.Present != race {
			t.Fatal("native cgo source selection", race, got)
		}
	}
	req.Family.Declarations[0].Hook.Context = ""
	req.Contexts = []spec.SourceContext{{OS: "linux", Arch: runtime.GOARCH, Race: true}}
	if get(collect(), "declaration").Present {
		t.Fatal("translated race sources enabled cgo")
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(data) != string(mod) {
		t.Fatal("caller module edited", err)
	}
	if err := c.Recheck(t.Context()); err != nil {
		t.Fatal(err)
	}
}
