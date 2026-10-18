package buildcfg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestConfigOverridesReplaceAndOwnFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gosim.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"toolchain":"go1.25.14","features":["rt","net"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	version, features := "go1.26.9", []string{"fs", "rt"}
	cfg, err := Load(path, Overrides{Toolchain: &version, Features: &features})
	if err != nil {
		t.Fatal(err)
	}
	want := buildspec.Config{Schema: 1, Toolchain: version, Features: []string{"rt", "fs"}, SimulationOS: "linux"}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %#v, want %#v", cfg, want)
	}
	features[0] = "net"
	if !reflect.DeepEqual(cfg, want) {
		t.Fatal("caller mutated normalized config")
	}
	missing := []string{}
	if _, err := Load(path, Overrides{Features: &missing}); err == nil {
		t.Fatal("explicit empty override treated as absent")
	}
}

func TestConfigRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{
		``, `null`, `[]`, `{}`, `{"schema":2}`, `{"schema":1,"toolchain":"go1.26.9","features":["rt"],"unknown":true}`,
		`{"schema":1,"toolchain":"go1.26.9","features":["rt"]} {}`,
	} {
		t.Run(input, func(t *testing.T) {
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
		cfg := buildspec.Config{Schema: 1, Toolchain: "go1.26.9", Features: features}
		if _, err := Normalize(cfg); err == nil {
			t.Fatalf("accepted invalid features %v", features)
		}
	}
	for _, version := range []string{"", "auto", "go1.26", "go1.26.9+auto", "devel go1.26", "go1.27.0", "go1.26.09"} {
		if _, err := Normalize(buildspec.Config{Schema: 1, Toolchain: version, Features: []string{"rt"}}); err == nil {
			t.Fatalf("accepted toolchain %q", version)
		}
	}
	if _, err := Normalize(buildspec.Config{Schema: 1, Toolchain: "go1.26.9", Features: []string{"rt"}, SimulationOS: "darwin"}); err == nil {
		t.Fatal("accepted simulated Darwin")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json"), Overrides{}); err == nil {
		t.Fatal("ignored explicit missing config")
	}
	if _, err := Load("", Overrides{}); err == nil {
		t.Fatal("silently selected full features")
	}
	version, features := "go1.26.9", []string{"net", "fs", "rt"}
	if cfg, err := Load("", Overrides{Toolchain: &version, Features: &features}); err != nil || !reflect.DeepEqual(cfg.Features, []string{"rt", "fs", "net"}) {
		t.Fatalf("complete CLI-only config: %#v, %v", cfg, err)
	}
}
