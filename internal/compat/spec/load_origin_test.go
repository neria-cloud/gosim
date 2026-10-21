package spec

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadOriginValidationAndIsolation(t *testing.T) {
	good := Origin{Schema: 2, Family: "1.25", BaselineToolchain: "go1.25.14", Files: []SourceFile{{Path: "src/runtime/runtime.go", Digest: strings.Repeat("a", 64)}}}
	encoded, _ := json.Marshal(good)
	files := fstest.MapFS{"go125.origin.json": &fstest.MapFile{Data: encoded}}
	first, err := LoadOrigin(files, "go125")
	if err != nil {
		t.Fatal(err)
	}
	first.Files[0].Path = "mutated"
	second, err := LoadOrigin(files, "go125")
	if err != nil || second.Files[0].Path != good.Files[0].Path {
		t.Fatal("origin state leaked", err)
	}
	for _, id := range []string{"../go125", "go125/../go125", "go1025", "go126", "go125.origin", "go124"} {
		if _, err := LoadOrigin(files, id); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	for _, mutate := range []func(*Origin){
		func(o *Origin) { o.Family = "1.26" },
		func(o *Origin) { o.BaselineToolchain = "go1.26.9" },
		func(o *Origin) { o.BaselineToolchain = "go1.25rc1" },
		func(o *Origin) { o.Schema = 1 },
		func(o *Origin) { o.Files = nil },
		func(o *Origin) { o.Files[0].Path = "src/../escape" },
		func(o *Origin) { o.Files[0].Digest = "invalid" },
		func(o *Origin) { o.Files = append(o.Files, o.Files[0]) },
	} {
		var bad Origin
		if err := json.Unmarshal(encoded, &bad); err != nil {
			t.Fatal(err)
		}
		mutate(&bad)
		data, _ := json.Marshal(bad)
		if _, err := LoadOrigin(fstest.MapFS{"go125.origin.json": &fstest.MapFile{Data: data}}, "go125"); err == nil {
			t.Fatalf("accepted invalid origin %+v", bad)
		}
	}
}
