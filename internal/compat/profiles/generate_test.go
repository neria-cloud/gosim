package profiles

import (
	"context"
	"encoding/json"
	"go/build/constraint"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func metadataFixture(t *testing.T) (string, map[string]spec.FamilyDefinition, func(string, []byte)) {
	t.Helper()
	root := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	defs := map[string]spec.FamilyDefinition{
		"1.25": {Schema: 1, Family: "1.25", Supported: true},
		"1.26": {Schema: 1, Family: "1.26", Parent: "1.25", Supported: false},
		"1.27": {Schema: 1, Family: "1.27", Parent: "1.26", Supported: true},
	}
	for _, d := range defs {
		data, _ := json.Marshal(d)
		write("internal/compat/profiles/go"+strings.ReplaceAll(d.Family, ".", "")+".json", data)
	}
	write("internal/hostabi/native.go", []byte("package hostabi\n"))
	write("internal/buildspec/family.go", []byte("package buildspec\n"))
	return root, defs, write
}

func TestMetadataBootstrapGapsAndRepair(t *testing.T) {
	root, defs, write := metadataFixture(t)
	ctx := t.Context()
	if err := GenerateMetadata(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	if err := GenerateMetadata(ctx, root, true); err == nil || !strings.Contains(err.Error(), "origin.json") {
		t.Fatalf("missing origin accepted: %v", err)
	}
	for _, d := range defs {
		if !d.Supported {
			continue
		}
		data, _ := json.Marshal(spec.Origin{Schema: 2, Family: d.Family, BaselineToolchain: "go" + d.Family + ".0", Files: []spec.SourceFile{{Path: "src/runtime/runtime.go", Digest: strings.Repeat("a", 64)}}})
		write("internal/compat/profiles/go"+strings.ReplaceAll(d.Family, ".", "")+".origin.json", data)
	}
	if err := GenerateMetadata(ctx, root, true); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"internal/compat/selected", "internal/compat/host", "internal/hostabi"} {
		data, err := os.ReadFile(filepath.Join(root, dir, "unsupported.go"))
		if err != nil {
			t.Fatal(err)
		}
		var expr constraint.Expr
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "//go:build ") {
				expr, err = constraint.Parse(line)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if expr == nil {
			t.Fatal("missing guard")
		}
		for _, minor := range []int{24, 25, 26, 27, 28} {
			rejected := expr.Eval(func(tag string) bool {
				for n := 1; n <= minor; n++ {
					if tag == "go1."+strconv.Itoa(n) {
						return true
					}
				}
				return false
			})
			if rejected != (minor != 25 && minor != 27) {
				t.Fatalf("guard %s minor %d", dir, minor)
			}
		}
	}
	filename := filepath.Join(root, "internal/compat/profiles/revisions_gen.go")
	before, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := GenerateMetadata(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(filename)
	if err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("unchanged metadata rewritten", err)
	}
	write("internal/buildspec/families_gen.go", []byte("package buildspec\n"))
	if err := GenerateMetadata(ctx, root, true); err == nil || !strings.Contains(err.Error(), "families_gen.go") {
		t.Fatalf("stale support not checked first: %v", err)
	}
	if err := GenerateMetadata(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	d := defs["1.27"]
	d.Supported = false
	data, _ := json.Marshal(d)
	write("internal/compat/profiles/go127.json", data)
	if err := GenerateMetadata(ctx, root, true); err == nil {
		t.Fatal("retired selector accepted")
	}
	if err := GenerateMetadata(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal/compat/selected/go127.go")); !os.IsNotExist(err) {
		t.Fatal("retired selector survived", err)
	}
	if err := GenerateMetadata(ctx, root, true); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := GenerateMetadata(canceled, root, false); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestConservativeRevisionAndOriginIndependence(t *testing.T) {
	root, _, write := metadataFixture(t)
	revision := func() string {
		t.Helper()
		value, err := spec.SourceRevision(root, "go127")
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	write("internal/compat/hooks/rt/store_go126.go", []byte("original"))
	before := revision()
	write("internal/compat/hooks/rt/store_go126.go", []byte("changed"))
	if revision() == before {
		t.Fatal("third family ignores inherited source variant")
	}
	before = revision()
	for _, name := range []string{"internal/compat/profiles/go125.origin.json", "internal/compat/profiles/go127.origin.json", "internal/compat/profiles/evidence.json", "internal/compat/profiles/revisions_gen.go"} {
		write(name, []byte("changed"))
	}
	if revision() != before {
		t.Fatal("provenance/history/revision fed back into adapter hash")
	}
	write("internal/compat/profiles/nested/evidence.json", []byte("executable metadata"))
	if revision() == before {
		t.Fatal("overbroad history exclusion")
	}
	if err := os.Symlink(filepath.Join(root, "internal/hostabi/native.go"), filepath.Join(root, "internal/compat/escape.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := spec.SourceRevision(root, "go127"); err == nil {
		t.Fatal("symlink input accepted")
	}
}

func TestRevisionTableMatchesCurrentSources(t *testing.T) {
	for _, id := range []string{"go125", "go126"} {
		want, err := spec.SourceRevision("../../..", id)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Revision(id)
		if err != nil || got != want {
			t.Fatalf("stale revision %s: %v", id, err)
		}
	}
}

func TestMetadataReplacementPreservesCompleteFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "generated.go")
	if err := os.WriteFile(path, []byte("old complete content"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := replaceMetadata(ctx, path, []byte("new complete content")); err == nil {
		t.Fatal("cancellation ignored")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "old complete content" {
		t.Fatal("canceled write destroyed old file", err)
	}
	if err := replaceMetadata(t.Context(), path, []byte("new complete content")); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "new complete content" {
		t.Fatal("replacement failed", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary output leaked", entries, err)
	}
}

func TestRevisionRejectsStraysAndUsesExactGeneratedExclusion(t *testing.T) {
	root, _, write := metadataFixture(t)
	before, err := spec.SourceRevision(root, "go127")
	if err != nil {
		t.Fatal(err)
	}
	write("internal/compat/hooks/revisions_gen.go", []byte("executable source"))
	after, err := spec.SourceRevision(root, "go127")
	if err != nil || after == before {
		t.Fatal("overbroad generated exclusion", err)
	}
	for _, name := range []string{".DS_Store", ".source.go.swp", "source.go~"} {
		path := "internal/compat/hooks/" + name
		write(path, []byte("stray"))
		if _, err := spec.SourceRevision(root, "go127"); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatal("stray input not diagnosed", name, err)
		}
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
}
