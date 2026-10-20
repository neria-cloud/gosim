package buildspec_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestPrivateModulePreservesWorkspaceSelection(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), fixtureModuleDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), fixtureModuleFilePerm); err != nil {
			t.Fatal(err)
		}
	}
	write("app/go.mod", "module example.org/app\ngo 1.25\nrequire example.org/foo/v2 v2.3.4\n")
	write("app/app.go", "package app\nimport \"example.org/foo/v2\"\nconst Value = foo.Value\n")
	write("foo/go.mod", "module example.org/foo/v2\ngo 1.25\nrequire example.org/dep v0.3.0\n")
	write("foo/foo.go", "package foo\nimport \"example.org/dep\"\nconst Value = dep.Value\n")
	write("dep/go.mod", "module example.org/dep\ngo 1.25\n")
	write("dep/dep.go", "package dep\nconst Value = 42\n")
	work := "go 1.25\nuse (\n./app\n./foo\n)\nreplace example.org/foo/v2 v2.3.4 => ./foo\nreplace example.org/dep => ./dep\n"
	write("go.work", work)
	r, err := buildcfg.ResolveRunningProcess(t.Context(), filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	r.Context.GoWork = filepath.Join(root, "go.work")
	input, err := buildspec.ResolveModuleGraph(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	files, err := buildspec.ModuleFiles(r, input, fixturePrivateModule)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		write(filepath.Join(fixtureOutputDir, name), string(content))
	}
	flat := r
	flat.Context = buildspec.ModuleContext{Directory: filepath.Join(root, fixtureOutputDir), GoWork: "off", GoMod: filepath.Join(root, fixtureOutputDir, "go.mod")}
	output, err := buildspec.ResolveModuleGraph(t.Context(), flat)
	if err != nil {
		t.Fatal(err)
	}
	if err := buildspec.MatchModuleGraph(input, output, fixturePrivateModule); err != nil {
		t.Fatal(err)
	}
	cmd := buildspec.ApplicationCommand(t.Context(), flat, "list", "-mod=readonly", "-deps", "example.org/app")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("load flattened selection: %v\n%s\n%s", err, out, files["go.mod"])
	}
	if data, err := os.ReadFile(filepath.Join(root, "go.work")); err != nil || string(data) != work {
		t.Fatal("workspace edited", err)
	}
	if data, err := os.ReadFile(flat.Context.GoMod); err != nil || !bytes.Equal(data, files["go.mod"]) {
		t.Fatal("generated module required hidden updates", err)
	}
	for i, m := range output {
		if m.Path != "example.org/dep" {
			continue
		}
		changed := slices.Clone(output)
		changed[i].Version = "v0.4.0"
		if err := buildspec.MatchModuleGraph(input, changed, fixturePrivateModule); err == nil {
			t.Fatal("accepted version change")
		}
		changed = slices.Clone(output)
		replacement := *changed[i].Replace
		replacement.Dir = filepath.Join(root, "foo")
		changed[i].Replace = &replacement
		if err := buildspec.MatchModuleGraph(input, changed, fixturePrivateModule); err == nil {
			t.Fatal("accepted replacement change")
		}
	}
	if err := buildspec.MatchModuleGraph(input, output[:1], fixturePrivateModule); err == nil {
		t.Fatal("accepted missing modules")
	}
	if err := buildspec.MatchModuleGraph(input, append(slices.Clone(output), buildspec.Module{Path: "example.org/unselected"}), fixturePrivateModule); err == nil {
		t.Fatal("accepted extra module")
	}
}

func TestPrivateModuleDoesNotExpandPrunedDependencies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(name, content string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), fixtureModuleDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), fixtureModuleFilePerm); err != nil {
			t.Fatal(err)
		}
	}
	write("app/go.mod", `module example.org/app
go 1.25
require example.org/framework v0.0.0
replace example.org/framework => ../framework
replace example.org/dep => ../dep
replace example.org/unneeded => ../unneeded
`)
	write("framework/go.mod", "module example.org/framework\ngo 1.25\nrequire example.org/dep v0.0.0\n")
	write("dep/go.mod", "module example.org/dep\ngo 1.25\nrequire example.org/unneeded v0.0.0\n")
	write("unneeded/go.mod", "module example.org/unneeded\ngo 1.25\n")
	r, err := buildcfg.ResolveRunningProcess(t.Context(), filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := buildspec.ResolveModuleGraph(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range input {
		if m.Path == "example.org/unneeded" {
			t.Fatal("fixture does not exercise a pruned dependency")
		}
	}
	files, err := buildspec.ModuleFiles(r, input, fixturePrivateModule)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		write(filepath.Join(fixtureOutputDir, name), string(data))
	}
	flat := r
	flat.Context = buildspec.ModuleContext{Directory: filepath.Join(root, fixtureOutputDir), GoWork: "off", GoMod: filepath.Join(root, fixtureOutputDir, "go.mod")}
	output, err := buildspec.ResolveModuleGraph(t.Context(), flat)
	if err != nil {
		t.Fatal(err)
	}
	if err := buildspec.MatchModuleGraph(input, output, fixturePrivateModule); err != nil {
		t.Fatal(err)
	}
}
