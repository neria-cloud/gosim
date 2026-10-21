package buildcfg

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/testutil"
)

func TestRealUnregisteredCompilerAuditCLI(t *testing.T) {
	v, err := buildspec.ParseGoVersion(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	if v.Family() != "1.26" {
		t.Skip("candidate-specific fixture executes only under the running Go 1.26 compiler")
	}
	t.Parallel()
	start := time.Now()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	r, err := ResolveRunningProcess(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	root := testutil.CopySource(t, "../..")
	definitionPath := filepath.Join(root, "internal/compat/profiles/go126.json")
	data, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := spec.DecodeDefinition(data)
	if err != nil {
		t.Fatal(err)
	}
	definition.Supported = false
	data, err = json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(definitionPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// This initial tool still has the old registry; maintenance reads definitions
	// from disk and writes the omitted-family registry before the harness builds.
	generate := buildspec.OutputCommand(ctx, r.Toolchain, root, "run", "-ldflags=-checklinkname=0", "-tags=linkname", "./cmd/gosim-gen", "profiles", "-root=.")
	if out, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("prepare fixture: %v\n%s", err, out)
	}
	registry, err := os.ReadFile(filepath.Join(root, "internal/buildspec/families_gen.go"))
	if err != nil || strings.Contains(string(registry), `"1.26"`) {
		t.Fatal("fixture registry not omitted", err)
	}
	binary := filepath.Join(t.TempDir(), "gosim-gen")
	build := buildspec.OutputCommand(ctx, r.Toolchain, root, "build", "-ldflags=-checklinkname=0", "-tags=linkname", "-o", binary, "./cmd/gosim-gen")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, out)
	}
	inputs := map[string][]byte{}
	for _, name := range []string{"go.mod", "go.sum"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		inputs[name] = b
	}
	output := filepath.Join(root, "candidate-comparison")
	cmd := exec.CommandContext(ctx, binary, "compare", "-parent=go125", "-arches="+runtime.GOARCH, "-output="+output)
	cmd.Dir = root
	// Bound child temporary modules to the fixture, including timeout cleanup.
	cmd.Env = append(withoutEnv(build.Env, "TMPDIR"), "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unregistered CLI compare: %v\n%s", err, out)
	}
	report, err := os.ReadFile(filepath.Join(output, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Complete bool
		APIDiff  string `json:"api_diff"`
		Findings []struct{ Kind string }
	}
	if err := json.Unmarshal(report, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Complete || result.APIDiff != "skipped-no-parent" {
		t.Fatal(result.Complete, result.APIDiff)
	}
	kinds := map[string]int{}
	for _, f := range result.Findings {
		kinds[f.Kind]++
	}
	if kinds["signature"] == 0 || kinds["source"] == 0 {
		t.Fatal("no actual source parsing evidence", kinds)
	}
	ordinary := exec.CommandContext(ctx, binary, "-profile=go126", "-arches="+runtime.GOARCH, "-output="+root, "-check")
	ordinary.Dir, ordinary.Env = root, cmd.Env
	rejected, err := ordinary.CombinedOutput()
	if err == nil || !strings.Contains(string(rejected), "unsupported Go") {
		t.Fatalf("ordinary fixture admission: %v\n%s", err, rejected)
	}
	for name, want := range inputs {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("caller module changed", name, err)
		}
	}
	t.Logf("actual %s omitted-registry CLI comparison in %s: %s", runtime.Version(), time.Since(start), out)
}
