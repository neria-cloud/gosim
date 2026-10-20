package buildcfg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestConfigOverridesReplaceAndOwnFeatures(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gosim.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"toolchain":"go1.25.1","features":["rt","net"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	version, features := "go1.26.1", []string{"fs", "rt"}
	got, err := Load(path, Overrides{Toolchain: &version, Features: &features})
	if err != nil {
		t.Fatal(err)
	}
	want := ConfigRequest{Config: buildspec.Config{Schema: 2, Features: []string{"rt", "fs"}, SimulationOS: "linux"}, ExplicitToolchain: version, Legacy: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	features[0] = "net"
	if !reflect.DeepEqual(got, want) {
		t.Fatal("caller mutated normalized config")
	}
	missing := []string{}
	if _, err := Load(path, Overrides{Features: &missing}); err == nil {
		t.Fatal("explicit empty override treated as absent")
	}
}

func TestConfigDefaultUsesGoResolution(t *testing.T) {
	t.Parallel()
	got, err := Load("", Overrides{})
	if err != nil || got.ExplicitToolchain != "" || got.Legacy || !reflect.DeepEqual(got.Config.Features, []string{"rt", "fs", "net"}) {
		t.Fatalf("default config: %+v, %v", got, err)
	}
	path := filepath.Join(t.TempDir(), "gosim.json")
	if err := os.WriteFile(path, []byte(`{"schema":2,"features":["rt"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path, Overrides{})
	if err != nil || got.ExplicitToolchain != "" || got.Config.SimulationOS != "linux" {
		t.Fatalf("schema 2: %+v, %v", got, err)
	}
}

func TestConfigRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		``, `null`, `[]`, `{}`, `{"schema":3}`, `{"schema":2}`,
		`{"schema":2,"toolchain":"go1.26.9","features":["rt"]}`,
		`{"schema":1,"features":["rt"]}`, `{"schema":1,"toolchain":"","features":["rt"]}`,
		`{"schema":1,"toolchain":"go1.26.9","features":["rt"],"unknown":true}`,
		`{"schema":2,"features":["rt"]} {}`,
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "gosim.json")
			if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path, Overrides{}); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
	for _, features := range [][]string{nil, {}, {"fs"}, {"net"}, {"fs", "net"}, {"rt", "fs", "fs"}, {"rt", "RT"}, {"rt", ""}} {
		if _, err := Normalize(buildspec.Config{Schema: 2, Features: features}); err == nil {
			t.Fatalf("accepted features %v", features)
		}
	}
	for _, version := range []string{"auto", "go1.26", "go1.26.9+auto", "devel go1.26", "go1.27.0", "go1.26.09"} {
		if _, err := Load("", Overrides{Toolchain: &version}); err == nil {
			t.Fatalf("accepted override %q", version)
		}
	}
	if _, err := Normalize(buildspec.Config{Schema: 2, Features: []string{"rt"}, SimulationOS: "darwin"}); err == nil {
		t.Fatal("accepted simulated Darwin")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json"), Overrides{}); err == nil {
		t.Fatal("ignored explicit missing config")
	}
}
