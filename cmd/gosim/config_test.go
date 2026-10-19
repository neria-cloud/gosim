package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestCommandCertificationParity(t *testing.T) {
	t.Parallel()
	config := buildspec.Config{Schema: 1, Toolchain: runtime.Version(), Features: []string{"rt"}, SimulationOS: "linux"}
	data, _ := json.Marshal(config)
	path := filepath.Join(t.TempDir(), "gosim.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	flags := BuildFlags{ConfigPath: path}
	for _, command := range []string{"translate", "test", "build-tests", "debug", "prepare-selftest", "lookup-test", "resolve-build"} {
		t.Run(command, func(t *testing.T) {
			if _, err := resolveBuild(command, flags); err == nil || !strings.Contains(err.Error(), "pending") {
				t.Fatalf("normal command accepted pending tuple: %v", err)
			}
		})
	}
	selected, err := resolveBuild("internal-candidate", flags)
	if err != nil || selected.Identity.Certification != "candidate" {
		t.Fatal(selected, err)
	}
	features := []string{"rt", "net"}
	flags.Overrides.Features = &features
	selected, err = resolveBuild("internal-candidate", flags)
	if err != nil || strings.Join(selected.Identity.Features, ",") != "rt,net" {
		t.Fatal("override did not replace features", selected, err)
	}
}
