package routegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestSelectedGeneratedDestinations(t *testing.T) {
	root := t.TempDir()
	pkg := adapterModule + "/internal/compat/hooks/kernel"
	dir := filepath.Join(root, "internal/compat/hooks/kernel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, tag string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("//go:build "+tag+"\n\npackage kernel\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("syscalls_old_amd64.go", "go1.25 && !go1.26")
	write("syscalls_new_amd64.go", "go1.26")
	write("syscalls_arm64.go", "go1.25")
	write("syscalls_test.go", "go1.25")
	for family, want := range map[string]string{"1.25": "old", "1.26": "new", "1.27": "new"} {
		path, err := selectedGeneratedPath(root, pkg, "syscalls", spec.SourceContext{OS: "linux", Arch: "amd64"}, family)
		if err != nil || !strings.HasSuffix(path, "syscalls_"+want+"_amd64.go") {
			t.Fatal(family, path, err)
		}
		constraint, err := authoredConstraint(root, path)
		if err != nil || family == "1.27" && constraint != "//go:build go1.26\n" {
			t.Fatal("rewrote cumulative constraint", constraint, err)
		}
	}
	if _, err := selectedGeneratedPath(root, pkg, "layouts", spec.SourceContext{OS: "linux", Arch: "arm64"}, "1.26"); err == nil {
		t.Fatal("invented missing destination")
	}
	write("syscalls_extra.go", "go1.26")
	if _, err := selectedGeneratedPath(root, pkg, "syscalls", spec.SourceContext{OS: "linux", Arch: "arm64"}, "1.26"); err == nil {
		t.Fatal("ambiguous destination accepted")
	}
}

func TestGenerationRejectsDifferentRunningCompiler(t *testing.T) {
	tc, p, _ := currentGeneration(t)
	tc.Version = "go1.26.9"
	if p.ID == "go126" {
		tc.Version = "go1.25.14"
	}
	out := t.TempDir()
	if err := GenerateProfile(tc, p.ID, []string{"arm64"}, out, GenerateOptions{}); err == nil || !strings.Contains(err.Error(), "worker is") {
		t.Fatal("cross-compiler generation accepted", err)
	}
	if len(readOutputTree(t, out)) != 0 {
		t.Fatal("failed compiler check wrote outputs")
	}
}
