package buildrun

import (
	"slices"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func graphFixture() []compiledPackage {
	m := gosimtool.Module
	tr := "translated/" + gosimtool.ReplaceSpecialPackages(m)
	return []compiledPackage{
		{"translated/app", []string{m + "/rt/runtime", m + "/rt/runtime/abi", tr + "/internal_/compat/shims/reflect"}},
		{tr + "/internal_/compat/shims/reflect", []string{"reflect", m + "/rt/runtime/abi"}},
		{m + "/rt/runtime", []string{m + "/rt/runtime/abi", m + "/internal/hostabi"}},
		{m + "/rt/runtime/abi", nil},
		{m + "/internal/hostabi", []string{m + "/internal/compat/host", m + "/internal/compat/selected"}},
		{m + "/internal/compat/host", nil},
		{m + "/internal/compat/selected", []string{m + "/internal/compat/profiles"}},
		{m + "/internal/compat/profiles", []string{m + "/internal/compat/spec"}},
		{m + "/internal/compat/spec", []string{m + "/internal/buildspec"}},
		{m + "/internal/buildspec", nil},
	}
}

func TestCompiledNativeIslands(t *testing.T) {
	id := buildspec.Identity{Features: []string{"rt"}}
	if err := validateCompiledGraph(id, graphFixture()); err != nil {
		t.Fatal(err)
	}
	m := gosimtool.Module
	tr := "translated/" + gosimtool.ReplaceSpecialPackages(m)
	for _, target := range []string{m, m + "/internal/hostabi", m + "/internal/buildspec", m + "/rt/runtime/private", m + "/internal/compat/host", m + "/internal/compat/profiles", m + "/internal/compat/spec", m + "/internal/compat/hooks/rt", m + "/internal/compat/shims/reflect"} {
		graph := graphFixture()
		graph[0].Imports = append(graph[0].Imports, target+" [translated/app.test]")
		if err := validateCompiledGraph(id, graph); err == nil || !strings.Contains(err.Error(), "native dependency") {
			t.Fatalf("native escape %s: %v", target, err)
		}
	}
	for _, path := range []string{tr + "/internal_/compat/host", tr + "/internal_/compat/spec", tr + "/internal_/compat/audit", tr + "/internal_/compat/go125/hooks/rt", m + "/internal/compat/audit", m + "/internal/compat/go126/hooks/rt", m + "/internal/compat/unknown"} {
		graph := append(graphFixture(), compiledPackage{ImportPath: path})
		if err := validateCompiledGraph(id, graph); err == nil {
			t.Fatal("invalid compatibility node", path)
		}
	}
	for _, path := range []string{m + "/internal/compat/hooks/rt", m + "/internal/compat/shims/reflect"} {
		graph := graphFixture()
		graph[2].Imports = append(graph[2].Imports, path)
		graph = append(graph, compiledPackage{ImportPath: path})
		if err := validateCompiledGraph(id, graph); err == nil {
			t.Fatal("native translated-code node trusted through runtime", path)
		}
	}
	graph := graphFixture()
	graph[4].Imports = nil
	if err := validateCompiledGraph(id, graph); err == nil {
		t.Fatal("native metadata outside runtime closure admitted")
	}
	graph = graphFixture()
	graph[2].ImportPath += " [runtime.test]"
	if err := validateCompiledGraph(id, graph); err != nil {
		t.Fatal("test variant identity lost", err)
	}
}

func TestCompiledProviderIsolation(t *testing.T) {
	m := gosimtool.Module
	tr := "translated/" + gosimtool.ReplaceSpecialPackages(m)
	id := buildspec.Identity{Features: []string{"rt", "fs", "net"}}
	graph := append(graphFixture(), compiledPackage{ImportPath: tr + "/internal_/fs"}, compiledPackage{ImportPath: tr + "/internal_/net"})
	if err := validateCompiledGraph(id, graph); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{tr, tr + "/fs", tr + "/net", tr + "/rt", tr + "/internal_/contracts", tr + "/internal_/control/sub", tr + "/internal_/kernel", tr + "/internal_/net/model"} {
		candidate := append(slices.Clone(graph), compiledPackage{from, []string{tr + "/internal_/fs"}})
		if err := validateCompiledGraph(id, candidate); err == nil {
			t.Fatal("provider dependency escaped", from)
		}
	}
	for _, path := range []string{m + "/internal/fs", m + "/internal/net/model"} {
		if err := validateCompiledGraph(id, append(slices.Clone(graph), compiledPackage{ImportPath: path})); err == nil {
			t.Fatal("native provider escaped", path)
		}
	}
	if err := validateCompiledGraph(buildspec.Identity{Features: []string{"rt"}}, graph); err == nil {
		t.Fatal("disabled provider admitted")
	}
	if err := validateCompiledGraph(id, graphFixture()); err == nil {
		t.Fatal("missing selected providers admitted")
	}
}
