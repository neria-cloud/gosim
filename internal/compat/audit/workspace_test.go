package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/testutil"
)

func TestAuditWorkspaceInputsAndCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	root := testutil.CopySource(t, "../../..")
	external := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(external, "go.mod"), "module example.org/inspection\ngo 1.25\n")
	source := filepath.Join(external, "source.go")
	write(source, "package inspection\nfunc Hook(){}\n")
	work := filepath.Join(t.TempDir(), "go.work")
	workBytes := fmt.Sprintf("go 1.25.0\nuse (\n%q\n%q\n)\n", root, external)
	write(work, workBytes)
	t.Setenv("GOWORK", work)
	c, err := buildcfg.DiscoverInspection(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	parent := spec.ResolvedFamily{Declarations: []spec.Declaration{{Hook: spec.Hook{Package: "example.org/inspection", Symbol: "Hook"}}}}
	contexts := []spec.SourceContext{{OS: "linux", Arch: runtime.GOARCH}}
	w, err := prepareAuditWorkspace(ctx, c.Toolchain(), root, parent, contexts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
	if w.Root != root || w.Directory == root || len(w.ModuleInputs) == 0 {
		t.Fatal("invalid workspace", w)
	}
	out, err := spec.CollectSourceObservations(ctx, c.Toolchain(), w.Directory, spec.ObservationRequest{Family: parent, Contexts: contexts, IncludeAPIs: true})
	if err != nil || !out.Contexts[0].Complete {
		t.Fatal(out, err)
	}
	if err := w.Recheck(ctx); err != nil {
		t.Fatal(err)
	}
	// New family data and generated support do not alter the inspector/module identities.
	write(filepath.Join(root, "internal/compat/profiles/go127.json"), `{"schema":1,"family":"1.27","parent":"1.26","supported":true,"delta":{}}`)
	registry := filepath.Join(root, "internal/buildspec/families_gen.go")
	oldRegistry, err := os.ReadFile(registry)
	if err != nil {
		t.Fatal(err)
	}
	write(registry, string(oldRegistry)+"\n// generated target addition\n")
	if err := w.Recheck(ctx); err != nil {
		t.Fatal("onboarding staled source snapshot", err)
	}
	write(source, "package inspection\nfunc Hook(x int){}\n")
	if err := w.Recheck(ctx); err == nil || !strings.Contains(err.Error(), "module inputs changed") {
		t.Fatal("replacement drift not rejected", err)
	}
	write(source, "package inspection\nfunc Hook(){}\n")
	privateMod := filepath.Join(w.Directory, "go.mod")
	original, err := os.ReadFile(privateMod)
	if err != nil {
		t.Fatal(err)
	}
	write(privateMod, string(original)+"\n// mutation\n")
	if err := w.Recheck(ctx); err == nil || !strings.Contains(err.Error(), "private inspection module changed") {
		t.Fatal(err)
	}
	write(privateMod, string(original))
	implementation := filepath.Join(root, "internal/compat/audit/identity.go")
	oldCode, err := os.ReadFile(implementation)
	if err != nil {
		t.Fatal(err)
	}
	write(implementation, string(oldCode)+"\n// changed implementation\n")
	if err := w.Recheck(ctx); err == nil || !strings.Contains(err.Error(), "inspector implementation changed") {
		t.Fatal(err)
	}
	write(implementation, string(oldCode))
	if err := w.Recheck(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(work)
	if err != nil || string(data) != workBytes {
		t.Fatal("caller workspace rewritten", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.Directory); !os.IsNotExist(err) {
		t.Fatal("workspace retained", err)
	}
	// A failed source lookup cleans the already-created private module.
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	parent.Declarations[0].Hook.Package = "internal/gosim_missing_package"
	if _, err := prepareAuditWorkspace(ctx, c.Toolchain(), root, parent, contexts); err == nil {
		t.Fatal("bad source accepted")
	}
	entries, err := os.ReadDir(tmp)
	if err != nil || len(entries) != 0 {
		t.Fatal("preparation leaked temporary modules", entries, err)
	}
}

func TestPrivateModuleDirectivesAreTransient(t *testing.T) {
	root, write := identityFixture(t)
	write("go.mod", "module "+frameworkModule+"\ngo 1.25\n")
	modules := []buildspec.Module{{Path: frameworkModule, Main: true, Dir: root, GoMod: filepath.Join(root, "go.mod")}}
	var identities [][]buildspec.Revision
	for _, version := range []string{"go1.25.14", "go1.26.9"} {
		files, err := buildspec.SourceInspectionModuleFiles(buildspec.Resolution{Toolchain: buildspec.Toolchain{Version: version}}, modules, "gosim-hook-source")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(files["go.mod"]), strings.TrimPrefix(version, "go")) {
			t.Fatal("private directive lost exact compiler")
		}
		identity, err := moduleInputs(t.Context(), buildspec.Resolution{}, modules, map[string]string{frameworkModule: root})
		if err != nil {
			t.Fatal(err)
		}
		identities = append(identities, identity)
	}
	if !reflect.DeepEqual(identities[0], identities[1]) {
		t.Fatal("compiler-specific private directive stales parent snapshot")
	}
}
