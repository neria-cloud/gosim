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

func TestRealUnregisteredCompilerObservations(t *testing.T) {
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
	program := filepath.Join(root, "cmd/inspection-smoke/main.go")
	if err := os.MkdirAll(filepath.Dir(program), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, []byte(inspectionSmokeProgram), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "inspection-smoke")
	build := buildspec.OutputCommand(ctx, r.Toolchain, root, "build", "-ldflags=-checklinkname=0", "-tags=linkname", "-o", binary, "./cmd/inspection-smoke")
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
	cmd := exec.CommandContext(ctx, binary)
	cmd.Dir = root
	// Bound all child temporary modules to the fixture, including forced timeout cleanup.
	cmd.Env = append(withoutEnv(build.Env, "TMPDIR"), "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect unregistered compiler: %v\n%s", err, out)
	}
	var result struct {
		Version          string
		OrdinaryRejected bool
		Kinds            map[string]int
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if result.Version != runtime.Version() || !result.OrdinaryRejected {
		t.Fatal(result)
	}
	for _, kind := range []string{"declaration", "layout", "constant", "package", "source"} {
		if result.Kinds[kind] == 0 {
			t.Fatal("no observed "+kind, result)
		}
	}
	for name, want := range inputs {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("caller module changed", name, err)
		}
	}
	t.Logf("actual %s omitted-registry observation in %s: %s", runtime.Version(), time.Since(start), out)
}

const inspectionSmokeProgram = `package main
import (
 "context"
 "encoding/json"
 "fmt"
 "os"
 "runtime"
 "github.com/neria-cloud/gosim/internal/buildcfg"
 "github.com/neria-cloud/gosim/internal/buildspec"
 "github.com/neria-cloud/gosim/internal/compat/profiles"
 "github.com/neria-cloud/gosim/internal/compat/spec"
)
func main(){if err:=run();err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}}
func run()(result error){
 ctx:=context.Background()
 if buildspec.ValidateVersion(runtime.Version())==nil{return fmt.Errorf("ordinary fixture admission accepted")}
 if _,err:=buildcfg.Discover(ctx,"",runtime.Version());err==nil{return fmt.Errorf("ordinary discovery accepted")}
 c,err:=buildcfg.DiscoverInspection(ctx,"","");if err!=nil{return err}
 if buildspec.OutputCommand(ctx,c.Toolchain(),"","version").Err==nil{return fmt.Errorf("ordinary command accepted")}
 defs,err:=profiles.Definitions();if err!=nil{return err}
 if defs["1.26"].Supported{return fmt.Errorf("fixture definition still supported")}
 parent,err:=profiles.Resolve("1.25");if err!=nil{return err}
 origin,err:=spec.LoadOrigin(os.DirFS("internal/compat/profiles"),"go125");if err!=nil{return err}
 dir,cleanup,err:=spec.PrepareSourceInspectionContext(ctx,buildspec.Resolution{Toolchain:c.Toolchain()});if err!=nil{return err}
 defer func(){if err:=cleanup();err!=nil{result=err}}()
 observations,err:=spec.CollectSourceObservations(ctx,c.Toolchain(),dir,spec.ObservationRequest{Family:parent,Origin:origin,Contexts:[]spec.SourceContext{{OS:"linux",Arch:runtime.GOARCH}}});if err!=nil{return err}
 if len(observations.APIPackages)!=0 || len(observations.Contexts)!=1 || !observations.Contexts[0].Complete{return fmt.Errorf("wrong observation coverage")}
 kinds:=map[string]int{};for _,r:=range observations.Records{if r.Present{kinds[r.Kind]++}}
 if err:=c.Recheck(ctx);err!=nil{return err}
 return json.NewEncoder(os.Stdout).Encode(struct{Version string;OrdinaryRejected bool;Kinds map[string]int}{runtime.Version(),true,kinds})
}
`
