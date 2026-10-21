package spec

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

const emptyDefinition = `{"schema":1,"family":"1.25","supported":false,"delta":{}}`

func TestDecodeDefinition(t *testing.T) {
	valid := `{"schema":1,"family":"1.27","parent":"1.26","supported":true,"delta":{"declarations":{"add":[{"hook":{"package":"runtime","symbol":"foo","signature":"func()","target":"github.com/neria-cloud/gosim/internal/compat/hooks/rt.Foo","owner":"rt"},"arches":["arm64","amd64"]}]},"architectures":{"amd64":{"layouts":{"add":[{"package":"runtime","type":"thing","size":16,"align":8,"fields":[{"name":"z","type":"int64","offset":0},{"name":"a","type":"int64","offset":8}]}]}}},"probes":{"add":[{"package":"runtime","symbol":"foo","phase":"init","operation":"foo","disposition":"modeled","feature":"rt","response":"ok","source":"src/runtime/foo.go","arguments":{"packages":[9007199254740993,-1]}}]}}}`
	d, err := DecodeDefinition([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Delta.Declarations.Add[0].Arches) != 0 {
		t.Fatal("common scope not normalized")
	}
	fields := d.Delta.Architectures["amd64"].Layouts.Add[0].Fields
	if fields[0].Name != "z" || fields[1].Name != "a" {
		t.Fatal("ordered layout payload changed")
	}
	if !reflect.DeepEqual(d.Delta.Probes.Add[0].Arguments["packages"], []int64{-1, 9007199254740993}) {
		t.Fatal("probe values lost precision or set normalization")
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeDefinition(data)
	if err != nil || !reflect.DeepEqual(again, d) {
		t.Fatal("normalized definition is not stable", err)
	}
	d, err = DecodeDefinition([]byte(emptyDefinition))
	if err != nil || d.Supported {
		t.Fatal("explicit false not accepted", err)
	}
}

func TestRejectDefinition(t *testing.T) {
	cases := []string{
		`null`, `[]`, emptyDefinition + `{}`, emptyDefinition + `garbage`,
		strings.Replace(emptyDefinition, `"schema":1`, `"schema":2`, 1),
		strings.Replace(emptyDefinition, `"schema":1`, `"schema":1,"schema":1`, 1),
		strings.Replace(emptyDefinition, `"supported":false,`, ``, 1),
		strings.Replace(emptyDefinition, `"supported":false`, `"supported":null`, 1),
		strings.Replace(emptyDefinition, `"1.25"`, `"1.025"`, 1),
		strings.Replace(emptyDefinition, `"1.25"`, `"1.25.14"`, 1),
		strings.Replace(emptyDefinition, `"1.25"`, `"go125"`, 1),
		strings.Replace(emptyDefinition, `"delta":{}`, `"delta":{"surprise":{}}`, 1),
		strings.Replace(emptyDefinition, `"delta":{}`, `"delta":null`, 1),
		`{"kind":"draft","schema":1,"family":"1.27","supported":true,"delta":{}}`,
	}
	deltas := []string{
		`{"packages":{"add":[{"path":"fmt","mode":"translate","extra":true}]}}`,
		`{"packages":{"add":[{"path":"fmt","path":"os"}]}}`,
		`{"runtime_packages":{"add":[{"path":"../outside"}]}}`,
		`{"runtime_packages":{"add":[{"path":"/absolute"}]}}`,
		`{"runtime_packages":{"add":[{"path":"a//b"}]}}`,
		`{"runtime_packages":{"add":[{"path":"a\\b"}]}}`,
		`{"runtime_packages":{"add":[null]}}`,
		`{"runtime_packages":{"add":[{"path":"a\u0000b"}]}}`,
		`{"packages":{"remove":["fmt"]}}`,
		`{"native_imports":{"remove":[["fmt"]]}}`,
		`{"native_imports":{"remove":[["../bad","unsafe"]]}}`,
		`{"packages":{"remove":[[false]]}}`,
		`{"packages":{"remove":[[""]]}}`,
		`{"declarations":{"remove":[["","runtime","foo","mips"]]}}`,
		`{"declarations":{"remove":[["host-other","runtime","foo","*"]]}}`,
		`{"architectures":{"mips":{}}}`, `{"architectures":null}`,
		`{"bridges":{"add":[{"package":"x","rewrites":[{"symbol":"F","package":"../bad","target":"F"}]}]}}`,
		`{"probes":{"add":[{"package":"x","symbol":"F","phase":"init","operation":"F","arguments":{"x":[1,1]}}]}}`,
	}
	for _, delta := range deltas {
		cases = append(cases, strings.Replace(emptyDefinition, `"delta":{}`, `"delta":`+delta, 1))
	}
	for _, arches := range []string{`["amd64","amd64"]`, `["mips"]`, `null`, `[false]`} {
		delta := `{"declarations":{"add":[{"hook":{"package":"runtime","symbol":"F"},"arches":` + arches + `}]}}`
		cases = append(cases, strings.Replace(emptyDefinition, `"delta":{}`, `"delta":`+delta, 1))
	}
	for _, data := range cases {
		t.Run(data, func(t *testing.T) {
			if _, err := DecodeDefinition([]byte(data)); err == nil {
				t.Fatal("malformed definition accepted")
			}
		})
	}
}

func TestLoadDefinitions(t *testing.T) {
	files := fstest.MapFS{
		"go125.json":        {Data: []byte(emptyDefinition)},
		"go125.origin.json": {Data: []byte("not a definition")},
		"evidence.json":     {Data: []byte("not a definition")},
		"profiles.go":       {Data: []byte("package profiles")},
	}
	definitions, err := LoadDefinitions(files)
	if err != nil || len(definitions) != 1 {
		t.Fatal(definitions, err)
	}
	for _, name := range []string{"go126.json", "go0125.json", "go125.extra.json", "draft.json"} {
		bad := fstest.MapFS{name: {Data: []byte(emptyDefinition)}}
		if _, err := LoadDefinitions(bad); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}
