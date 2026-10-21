package buildcfg

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestRunningInspectionPreservesWorkspace(t *testing.T) {
	root := t.TempDir()
	inputs := map[string]string{
		"app/go.mod": "module example.org/app\ngo 1.25\nrequire example.org/dep v1.0.0\n",
		"app/app.go": "package app\nimport \"example.org/dep\"\nconst Value=dep.Value\n",
		"dep/go.mod": "module example.org/dep\ngo 1.25\n",
		"dep/dep.go": "package dep\nconst Value=42\n",
		"go.work":    "go 1.25\nuse ./app\nreplace example.org/dep => ./dep\n",
	}
	for name, body := range inputs {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOWORK", filepath.Join(root, "go.work"))
	c, err := DiscoverInspection(t.Context(), filepath.Join(root, "app"), runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	// InspectionCompiler exposes no mutable resolution. Preparation discovers
	// the caller workspace with fixed queries when only the directory is given.
	r := buildspec.Resolution{Toolchain: c.Toolchain(), Context: buildspec.ModuleContext{Directory: filepath.Join(root, "app")}}
	dir, cleanup, err := spec.PrepareSourceInspectionContext(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	cmd := buildspec.SourceInspectionCommand(t.Context(), c.Toolchain(), dir, "list", "-mod=mod", "-json", "-deps", "-tags=linkname", "example.org/app")
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), `"ImportPath": "example.org/dep"`) {
		t.Fatalf("%s: %v", out, err)
	}
	if err := c.Recheck(t.Context()); err != nil {
		t.Fatal(err)
	}
	for name, want := range inputs {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != want {
			t.Fatal("caller input modified", name, err)
		}
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("private module retained", err)
	}
}
