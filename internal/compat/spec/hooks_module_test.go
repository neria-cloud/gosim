package spec

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/mod/modfile"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestHookInspectionModulePreservesInputs(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	replacement := filepath.Join(root, "replacement")
	for _, dir := range []string{project, replacement} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	source := []byte("module example.com/app/v2\n\ngo 1.25.0\nrequire example.com/dep v1.0.0\nreplace example.com/dep => ../replacement\n")
	path := filepath.Join(project, "go.mod")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, "go.mod"), []byte("module example.com/dep\n\ngo 1.25.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, "dep.go"), []byte("package dep\nfunc Hook(x int) int{return x}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	tc := buildspec.Toolchain{Version: runtime.Version(), Executable: filepath.Join(runtime.GOROOT(), "bin/go"), GOROOT: runtime.GOROOT(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	dir, cleanup, err := hookInspectionModule(tc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range parsed.Replace {
		if r.Old.Path == "example.com/dep" {
			found = r.New.Path == replacement
		}
	}
	if !found {
		t.Fatal("relative replacement not preserved", string(data))
	}
	hooks := []Hook{{Package: "example.com/dep", Symbol: "Hook", Signature: "func(int) int"}}
	if err := VerifyHooks(tc, hooks); err != nil {
		t.Fatal(err)
	}
	hooks[0].Symbol = "missing"
	if err := VerifyHooks(tc, hooks); err == nil {
		t.Fatal("missing hook accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, source) {
		t.Fatal("application go.mod modified", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("inspection module retained", err)
	}
}
