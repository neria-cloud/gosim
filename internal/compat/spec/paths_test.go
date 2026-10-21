package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatPackageClassification(t *testing.T) {
	for suffix, want := range map[string]CompatPackageKind{
		"hooks/rt": CompatHooks, "shims/reflect": CompatShims, "host": CompatHost,
		"profiles": CompatMetadata, "selected": CompatMetadata, "spec": CompatMetadata, "audit": CompatTooling,
		"unknown": CompatUnknown, "go125/hooks/rt": CompatHooks, "go126/shims/testing": CompatShims,
		"go127/hooks/rt": CompatUnknown, "hooks/../host": CompatUnknown,
	} {
		for _, prefix := range []string{module + "/internal/compat/", "translated/" + module + "/internal_/compat/"} {
			if got := ClassifyCompatPackage(prefix + suffix); got != want {
				t.Fatalf("%s: got %d want %d", prefix+suffix, got, want)
			}
		}
	}
	for _, path := range []string{"fmt", "example/compat/hooks/rt", module + "/internal/compatible/hooks/rt"} {
		if ClassifyCompatPackage(path) != CompatOutside {
			t.Fatal("classified unrelated path", path)
		}
	}
}

func TestSharedCompatibilityPermissions(t *testing.T) {
	p, _ := fixture()
	shared := module + "/internal/compat/shims/reflect"
	m := PolicyManifest{NativeImports: []NativeImport{{From: shared, Package: "reflect", Reason: "exact reviewed bridge"}}}
	p.Packages = []PackagePolicy{{Path: "reflect", Mode: "translate", Reason: "fixture"}}
	if err := m.Validate(p); err != nil {
		t.Fatal(err)
	}
	if err := m.AllowNativeImport(p, shared, "reflect", true); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{module + "/internal/compat/shims/testing", "example/app", module + "/internal/compat/go125/shims/reflect"} {
		if m.AllowNativeImport(p, from, "reflect", true) == nil {
			t.Fatal("category granted another package native permission", from)
		}
	}
	for _, target := range []string{module + "/internal/compat/hooks/rt", module + "/internal/compat/host", module + "/internal/compat/spec"} {
		m.NativeImports = append(m.NativeImports, NativeImport{From: shared, Package: target, Reason: "forged"})
		if m.AllowNativeImport(p, shared, target, false) == nil || m.Validate(p) == nil {
			t.Fatal("native compatibility escape", target)
		}
	}
	for _, target := range []string{shared + ".F", module + "/internal/compat/hooks/rt.F", module + "/internal/compat/host.F"} {
		if err := p.checkPackage(target); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{module + "/internal/compat/go125/hooks/rt.F", module + "/internal/compat/profiles.F", module + "/internal/compat/unknown.F", "translated/" + module + "/internal_/compat/hooks/rt.F"} {
		if p.checkPackage(target) == nil {
			t.Fatal("nonselected/unknown target accepted", target)
		}
	}
}

func TestSharedRevisionInputs(t *testing.T) {
	root := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{"internal/compat/go125", "internal/compat/go126", "internal/hostabi", "internal/compat/selected", "internal/compat/spec", "internal/buildspec"} {
		write(dir+"/input.go", "package fixture")
	}
	revision := func() string {
		t.Helper()
		value, err := SourceRevision(root, "go125")
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := revision()
	for _, dir := range []string{"hooks/rt", "shims/testing", "host", "profiles", "audit"} {
		name := "internal/compat/" + dir + "/shared.go"
		write(name, "package shared")
		after := revision()
		if after == before {
			t.Fatal("shared input not hashed", dir)
		}
		before = after
		write(name, "package changed")
		after = revision()
		if after == before {
			t.Fatal("shared edit not hashed", dir)
		}
		before = after
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
		after = revision()
		if after == before {
			t.Fatal("shared removal not hashed", dir)
		}
		before = after
	}
	for _, name := range []string{"internal/compat/profiles/revisions_gen.go", "internal/compat/go125/profile/revision.go", "internal/compat/hooks/rt/shared_test.go", "internal/compat/shims/testing/testdata/input.go"} {
		write(name, strings.Repeat("x", 64))
	}
	if revision() != before {
		t.Fatal("tests/generated revision output fed into source hash")
	}
}
