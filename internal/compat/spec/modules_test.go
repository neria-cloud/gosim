package spec_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"

	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestPinnedModuleGraphAndSources(t *testing.T) {
	tc := currentToolchain()
	pins := p125.PolicyManifest().Modules
	if err := spec.VerifyModules(tc, "../../..", pins); err != nil {
		t.Fatal(err)
	}
	wrong := append([]spec.ModulePin(nil), pins...)
	wrong[0].Version = "v0.0.0"
	if err := spec.VerifyModules(tc, "../../..", wrong); err == nil {
		t.Fatal("wrong module version accepted")
	}
	if err := spec.VerifyModules(tc, "../../..", append(pins, pins[0])); err == nil {
		t.Fatal("duplicate pin accepted")
	}
	dir := t.TempDir()
	replacement := filepath.Join(dir, "replacement")
	if err := os.Mkdir(replacement, 0o755); err != nil {
		t.Fatal(err)
	}
	writeModuleFile(t, filepath.Join(replacement, "go.mod"), "module "+pins[0].Path+"\n\ngo 1.25\n")
	writeModuleFile(t, filepath.Join(dir, "go.mod"), "module example.test/pin\n\ngo 1.25\nrequire "+pins[0].Path+" "+pins[0].Version+"\nreplace "+pins[0].Path+" => ./replacement\n")
	if err := spec.VerifyModules(tc, dir, pins[:1]); err == nil || !strings.Contains(err.Error(), "pin mismatch") {
		t.Fatalf("replacement error: %v", err)
	}
}

func TestModuleContentTampering(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.go")
	mod := filepath.Join(dir, "go.mod")
	writeModuleFile(t, source, "package fixture\nconst Value = 1\n")
	writeModuleFile(t, mod, "module example.test/fixture\n\ngo 1.25\n")
	pin := spec.ModulePin{Path: "example.test/fixture", Version: "v1.0.0"}
	var err error
	pin.Sum, err = dirhash.HashDir(dir, pin.Path+"@"+pin.Version, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}
	pin.GoModSum, err = dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) { return os.Open(mod) })
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.VerifyModuleSource(pin, dir, mod); err != nil {
		t.Fatal(err)
	}
	writeModuleFile(t, source, "package fixture\nconst Value = 2\n")
	if err := spec.VerifyModuleSource(pin, dir, mod); err == nil || !strings.Contains(err.Error(), "modified module source") {
		t.Fatalf("changed source: %v", err)
	}
	writeModuleFile(t, source, "package fixture\nconst Value = 1\n")
	otherMod := filepath.Join(t.TempDir(), "go.mod")
	writeModuleFile(t, otherMod, "module example.test/substituted\n")
	if err := spec.VerifyModuleSource(pin, dir, otherMod); err == nil || !strings.Contains(err.Error(), "modified module definition") {
		t.Fatalf("changed definition: %v", err)
	}
	if err := spec.VerifyModuleSource(pin, "", ""); err == nil {
		t.Fatal("missing source accepted")
	}
}

func writeModuleFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
