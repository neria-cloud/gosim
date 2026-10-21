package audit

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func identityFixture(t *testing.T) (string, func(string, string)) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, write
}

func TestInspectorIdentityTracksAllSourceVariants(t *testing.T) {
	root, write := identityFixture(t)
	write("cmd/gosim-gen/main.go", "package main\nimport _ \"github.com/neria-cloud/gosim/internal/helper\"\n")
	write("internal/compat/audit/audit.go", "package audit\n")
	write("internal/helper/common.go", "package helper\nimport _ \"github.com/neria-cloud/gosim/internal/buildspec\"\n")
	write("internal/helper/new_go127.go", "//go:build go1.27\npackage helper\nimport _ \"github.com/neria-cloud/gosim/internal/extra\"\n")
	write("internal/extra/extra.go", "package extra\n")
	write("internal/buildspec/base.go", "package buildspec\n")
	digest := func() string {
		t.Helper()
		s, err := inspectorDigest(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := digest()
	for _, name := range []string{"internal/compat/profiles/go127.json", "internal/compat/profiles/go126.origin.json", "internal/compat/profiles/evidence.json", "internal/compat/profiles/revisions_gen.go", "internal/buildspec/families_gen.go", "internal/helper/example_test.go", "internal/extra/README.md"} {
		write(name, "excluded fixture")
	}
	if digest() != before {
		t.Fatal("data/support/test edit staled inspector")
	}
	write("internal/extra/extra.go", "package extra\nconst Future = 1\n")
	if digest() == before {
		t.Fatal("future variant dependency ignored")
	}
	before = digest()
	write("cmd/gosim-gen/main.go", "package main\nimport _ \"github.com/neria-cloud/gosim/internal/helper\"\nconst RequestRule = 1\n")
	if digest() == before {
		t.Fatal("request construction ignored")
	}
	if inspectorGenerated("internal/compat/selected/go125helper.go") {
		t.Fatal("authored source excluded as a selector")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := inspectorDigest(cancelled, root); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err := os.Remove(filepath.Join(root, "internal/extra/extra.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "internal/helper/common.go"), filepath.Join(root, "internal/extra/extra.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectorDigest(t.Context(), root); err == nil {
		t.Fatal("symlink implementation accepted")
	}
}

func TestModuleInputsExcludeFrameworkDataAndBindReplacements(t *testing.T) {
	root, write := identityFixture(t)
	write("framework/go.mod", "module "+frameworkModule+"\ngo 1.25\n")
	write("external/go.mod", "module example.org/external\ngo 1.25\n")
	write("external/source.go", "package external\nconst Value=42\n")
	write("go.work", "go 1.25\nuse ./framework\n")
	framework, external := filepath.Join(root, "framework"), filepath.Join(root, "external")
	modules := []buildspec.Module{{Path: frameworkModule, Main: true, Dir: framework, GoMod: filepath.Join(framework, "go.mod")}, {Path: "example.org/external", Version: "v1.0.0", Replace: &buildspec.Module{Path: external, Dir: external, GoMod: filepath.Join(external, "go.mod")}}}
	r := buildspec.Resolution{Context: buildspec.ModuleContext{GoWork: filepath.Join(root, "go.work")}}
	used := map[string]string{frameworkModule: framework, "example.org/external": external}
	inputs := func() []buildspec.Revision {
		t.Helper()
		out, err := moduleInputs(t.Context(), r, modules, used)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := inputs()
	write("framework/internal/compat/profiles/go127.json", "new target definition")
	write("framework/internal/buildspec/families_gen.go", "generated support")
	write("framework/.gosim/generated/go.mod", "go 1.26.9\n")
	if !reflect.DeepEqual(before, inputs()) {
		t.Fatal("whole-framework/private compiler directive leaked into module identity")
	}
	write("external/source.go", "package external\nconst Value=43\n")
	if reflect.DeepEqual(before, inputs()) {
		t.Fatal("replacement source drift ignored")
	}
	write("external/source.go", "package external\nconst Value=42\n")
	modules[1].Version = "v1.1.0"
	if reflect.DeepEqual(before, inputs()) {
		t.Fatal("graph version drift ignored")
	}
	modules[1].Version = "v1.0.0"
	write("framework/go.mod", "module "+frameworkModule+"\ngo 1.25\n// changed\n")
	if reflect.DeepEqual(before, inputs()) {
		t.Fatal("caller module edit ignored")
	}
	used["unselected"] = external
	if _, err := moduleInputs(t.Context(), r, modules, used); err == nil {
		t.Fatal("unselected source module accepted")
	}
	delete(used, "unselected")
	used["example.org/external"] = framework
	if _, err := moduleInputs(t.Context(), r, modules, used); err == nil {
		t.Fatal("replacement directory mismatch accepted")
	}
}

func TestModuleSourceDigestIndependentOfLocation(t *testing.T) {
	var digests []string
	for i := 0; i < 2; i++ {
		root, write := identityFixture(t)
		write("source.go", "package fixture\n")
		digest, err := moduleSourceDigest(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		digests = append(digests, digest)
	}
	if digests[0] != digests[1] {
		t.Fatal("paths bound into portable source identity")
	}
}
