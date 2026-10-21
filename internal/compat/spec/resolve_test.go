package spec

import (
	"encoding/json"
	"reflect"
	"testing"
)

func definitionFixture(t *testing.T, family, parent, delta string) FamilyDefinition {
	t.Helper()
	d, err := DecodeDefinition([]byte(`{"schema":1,"family":"` + family + `","parent":"` + parent + `","supported":true,"delta":` + delta + `}`))
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func chainFixture(t *testing.T) map[string]FamilyDefinition {
	t.Helper()
	return map[string]FamilyDefinition{
		"1.25": definitionFixture(t, "1.25", "", `{"runtime_packages":{"add":[{"path":"one"},{"path":"two"}]}}`),
		"1.26": definitionFixture(t, "1.26", "1.25", `{}`),
		"1.27": definitionFixture(t, "1.27", "1.26", `{"runtime_packages":{"remove":[["one"]],"add":[{"path":"three"}]}}`),
	}
}

func TestResolveCumulative(t *testing.T) {
	defs := chainFixture(t)
	retired := defs["1.26"]
	retired.Supported = false
	defs["1.26"] = retired
	out, err := ResolveDefinitions(defs, "1.27")
	if err != nil {
		t.Fatal(err)
	}
	if out.Family != "1.27" || out.ProfileID != "go127" || !out.Supported || !reflect.DeepEqual(out.Chain, []string{"1.25", "1.26", "1.27"}) || !reflect.DeepEqual(out.RuntimePackages, []PackageRef{{"three"}, {"two"}}) {
		t.Fatalf("wrong resolved family %+v", out)
	}
	defs["1.28"] = definitionFixture(t, "1.28", "1.27", `{}`)
	next, err := ResolveDefinitions(defs, "1.28")
	if err != nil || !reflect.DeepEqual(out.RuntimePackages, next.RuntimePackages) {
		t.Fatal("removal did not persist", err)
	}
	defs["1.29"] = definitionFixture(t, "1.29", "1.28", `{"runtime_packages":{"add":[{"path":"one"}]}}`)
	readd, err := ResolveDefinitions(defs, "1.29")
	if err != nil || len(readd.RuntimePackages) != 3 {
		t.Fatal("explicit re-add failed", err)
	}
	base := defs["1.25"]
	base.Delta.RuntimePackages.Add[0], base.Delta.RuntimePackages.Add[1] = base.Delta.RuntimePackages.Add[1], base.Delta.RuntimePackages.Add[0]
	defs["1.25"] = base
	reordered, err := ResolveDefinitions(defs, "1.27")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := out.Canonical()
	b, _ := reordered.Canonical()
	if string(a) != string(b) {
		t.Fatal("set order changed identity")
	}
	retained, err := ResolveDefinitions(defs, "1.26")
	if err != nil || retained.Supported {
		t.Fatal("retained unsupported ancestor unavailable", err)
	}
}

func TestResolveInvalidAncestry(t *testing.T) {
	for _, mutate := range []func(map[string]FamilyDefinition){
		func(d map[string]FamilyDefinition) { delete(d, "1.26") },
		func(d map[string]FamilyDefinition) { v := d["1.27"]; v.Parent = "1.25"; d["1.27"] = v },
		func(d map[string]FamilyDefinition) { v := d["1.25"]; v.Parent = "1.27"; d["1.25"] = v },
		func(d map[string]FamilyDefinition) { d["duplicate"] = d["1.25"] },
		func(d map[string]FamilyDefinition) {
			v := d["1.25"]
			v.Delta.RuntimePackages.Remove = [][]string{{"one"}}
			d["1.25"] = v
		},
	} {
		defs := chainFixture(t)
		mutate(defs)
		if _, err := ResolveDefinitions(defs, "1.27"); err == nil {
			t.Fatal("invalid ancestry/baseline accepted")
		}
	}
}

func TestApplyRecords(t *testing.T) {
	key := func(v PackagePolicy) []string { return []string{v.Path} }
	base := []PackagePolicy{{Path: "p", Mode: "translate", Reason: "old"}}
	replacement := PackagePolicy{Path: "p", Mode: "bridge", Reason: "new"}
	out, err := applyRecords(base, RecordDelta[PackagePolicy]{Replace: []PackagePolicy{replacement}}, key)
	if err != nil || !reflect.DeepEqual(out, []PackagePolicy{replacement}) {
		t.Fatal(out, err)
	}
	for _, delta := range []RecordDelta[PackagePolicy]{
		{Add: base}, {Replace: base}, {Remove: [][]string{{"absent"}}},
		{Replace: []PackagePolicy{{Path: "absent"}}},
		{Remove: [][]string{{"p"}}, Add: base},
		{Remove: [][]string{{"p"}, {"p"}}},
		{Replace: []PackagePolicy{replacement}, Add: base},
	} {
		if _, err := applyRecords(base, delta, key); err == nil {
			t.Fatalf("invalid delta accepted: %+v", delta)
		}
	}
	if tupleKey([]string{"a:b", "c"}) == tupleKey([]string{"a", "b:c"}) {
		t.Fatal("ambiguous tuple encoding")
	}
}

func TestResolveDeclarationScopes(t *testing.T) {
	hook := Hook{Package: "runtime", Symbol: "f", Signature: "func()", Target: module + "/internal/compat/hooks/rt.F", Owner: "rt"}
	base := FamilyDefinition{Schema: 1, Family: "1.25", Supported: true, Delta: ProfileDelta{Declarations: RecordDelta[Declaration]{Add: []Declaration{{Hook: hook}}}}}
	defs := map[string]FamilyDefinition{"1.25": base, "1.26": {Schema: 1, Family: "1.26", Parent: "1.25", Supported: true}}
	next := defs["1.26"]
	next.Delta.Declarations.Add = []Declaration{{Hook: hook, Arches: []string{"amd64"}}, {Hook: hook, Arches: []string{"arm64"}}}
	defs["1.26"] = next
	if _, err := ResolveDefinitions(defs, "1.26"); err == nil {
		t.Fatal("overlapping declarations accepted")
	}
	next.Delta.Declarations.Remove = [][]string{{"", "runtime", "f", "*"}}
	defs["1.26"] = next
	out, err := ResolveDefinitions(defs, "1.26")
	if err != nil || len(out.Declarations) != 2 {
		t.Fatal("split scope failed", err)
	}
	// Host declarations also participate in the host-race context.
	if err := validateDeclarationOverlap([]Declaration{{Hook: Hook{Context: "host", Package: "runtime", Symbol: "f"}}, {Hook: Hook{Context: "host-race", Package: "runtime", Symbol: "f"}}}); err == nil {
		t.Fatal("host/race overlap accepted")
	}
}

func TestResolvedCopies(t *testing.T) {
	base := definitionFixture(t, "1.25", "", `{"probes":{"add":[{"package":"runtime","symbol":"f","phase":"init","operation":"f","source":"src/runtime/x.go","arguments":{"x":[1,2]}}]},"architectures":{"arm64":{"layouts":{"add":[{"package":"runtime","type":"X","size":8,"align":8,"fields":[{"name":"a","type":"int64","offset":0}]}]}}}}`)
	defs := map[string]FamilyDefinition{"1.25": base}
	out, err := ResolveDefinitions(defs, "1.25")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(out)
	clone := out.Clone()
	clone.Probes[0].Arguments["x"][0] = 99
	clone.Architectures["arm64"].Layouts[0].Fields[0].Name = "mutated"
	clone.Chain[0] = "mutation"
	base.Delta.Probes.Add[0].Arguments["x"][0] = 55
	base.Delta.Architectures["arm64"].Layouts.Add[0].Fields[0].Name = "also-mutated"
	after, _ := json.Marshal(out)
	if string(before) != string(after) {
		t.Fatal("nested state leaked")
	}
}

func TestArchitectureRemovalPersists(t *testing.T) {
	defs := map[string]FamilyDefinition{
		"1.25": definitionFixture(t, "1.25", "", `{"architectures":{"amd64":{"constants":{"add":[{"package":"runtime","symbol":"X","value":"1"}]}}}}`),
		"1.26": definitionFixture(t, "1.26", "1.25", `{"architectures":{"amd64":{"constants":{"remove":[["runtime","X"]]}}}}`),
		"1.27": definitionFixture(t, "1.27", "1.26", `{}`),
	}
	out, err := ResolveDefinitions(defs, "1.27")
	if err != nil || len(out.Architectures["amd64"].Constants) != 0 {
		t.Fatal("architecture removal lost", out, err)
	}
}
