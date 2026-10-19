package compose

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func selection(t *testing.T, features ...string) buildspec.Selection {
	t.Helper()
	p := selected.Profile()
	patch := "go1.26.9"
	if p.ID == "go125" {
		patch = "go1.25.14"
	}
	d := strings.Repeat("a", 64)
	id := buildspec.Identity{
		Schema: 1, Toolchain: patch, Profile: p.ID, AdapterRevision: p.Revision, DistributionDigest: d, StdlibDigest: d, RuntimeRevision: d,
		HostOS: runtime.GOOS, HostArch: runtime.GOARCH, SimulationOS: "linux", SimulationArch: runtime.GOARCH, Features: features, Tags: []string{"linkname", "sim"}, ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d, Certification: "candidate",
	}
	for _, f := range features {
		if f != "rt" {
			id.ProviderRevisions = append(id.ProviderRevisions, buildspec.Revision{Name: f, Digest: d})
		}
	}
	s, err := p.Select(id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGeneratedImportSetsAndConcurrentPublication(t *testing.T) {
	for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		t.Run(strings.Join(features, ","), func(t *testing.T) {
			s := selection(t, features...)
			dir := t.TempDir()
			var wg sync.WaitGroup
			for range 10 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					out, err := Generate(s, dir)
					if err != nil {
						t.Error(err)
						return
					}
					if err := ValidateImports(out, s); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			data, err := os.ReadFile(filepath.Join(dir, "composition.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, feature := range []string{"fs", "net"} {
				path := `"` + gosimtool.Module + "/internal/" + feature + `"`
				if bytes.Contains(data, []byte(path)) != slices.Contains(features, feature) {
					t.Fatalf("wrong %s provider import", feature)
				}
				if bytes.Contains(data, []byte(feature+"provider.NewProvider()")) != slices.Contains(features, feature) {
					t.Fatalf("wrong %s constructor", feature)
				}
			}
			other := "go125"
			if s.Identity.Profile == other {
				other = "go126"
			}
			if bytes.Contains(data, []byte("/compat/"+other+"/")) || bytes.Contains(data, []byte("/internal/hostabi")) {
				t.Fatal("generated private/native family escape")
			}
			before, _ := os.Stat(filepath.Join(dir, "composition.go"))
			if _, err := Generate(s, dir); err != nil {
				t.Fatal(err)
			}
			after, _ := os.Stat(filepath.Join(dir, "composition.go"))
			if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
				t.Fatal("identical generation rewrote source")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 1 {
				t.Fatal("staging files leaked", entries)
			}
		})
	}
}

func TestRejectCompositionTampering(t *testing.T) {
	s := selection(t, "rt")
	for _, test := range []string{"imports", "body", "extra-source", "symlink", "digest", "traversal", "files"} {
		t.Run(test, func(t *testing.T) {
			dir := t.TempDir()
			out, err := Generate(s, dir)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "composition.go")
			data, _ := os.ReadFile(path)
			switch test {
			case "imports":
				data = bytes.Replace(data, []byte("import ("), []byte("import (\n_ \"net/http\""), 1)
				os.WriteFile(path, data, 0o644)
			case "body":
				data = append(data, []byte("\nfunc Extra() {}\n")...)
				os.WriteFile(path, data, 0o644)
			case "extra-source":
				os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package gosimcomposition"), 0o644)
			case "symlink":
				target := filepath.Join(t.TempDir(), "source")
				os.WriteFile(target, data, 0o644)
				os.Remove(path)
				os.Symlink(target, path)
			case "digest":
				out.IdentityDigest = strings.Repeat("b", 64)
			case "traversal":
				out.SourceDir = dir + "/../" + filepath.Base(dir)
			case "files":
				out.Files = []string{"../composition.go"}
			}
			if err := ValidateImports(out, s); err == nil {
				t.Fatal("tampered composition accepted")
			}
			if test == "imports" || test == "body" {
				if _, err := Generate(s, dir); err == nil {
					t.Fatal("changed existing source accepted")
				}
				after, _ := os.ReadFile(path)
				if !bytes.Equal(after, data) {
					t.Fatal("existing output overwritten")
				}
			}
		})
	}
	dir := t.TempDir()
	s.ProviderImports = []string{gosimtool.Module + "/internal/fs"}
	if _, err := Generate(s, dir); err == nil {
		t.Fatal("forged providers accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("invalid selection wrote files")
	}
	s = selection(t, "rt")
	for _, bad := range []string{"", "relative", filepath.Join(dir, "missing")} {
		if _, err := Generate(s, bad); err == nil {
			t.Fatalf("bad directory accepted %q", bad)
		}
	}
	symlink := filepath.Join(dir, "link")
	if err := os.Symlink(t.TempDir(), symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(s, symlink); err == nil {
		t.Fatal("symlink staging root accepted")
	}
}

// This executes generated composition with native input packages to verify the
// Driver/Engine boundary. Task 21.4 separately proves actual AST-translated code.
func TestNativeGeneratedDriver(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod := "module " + gosimtool.Module + "/.gosim/compositionprobe\n\ngo 1.25.3\nrequire " + gosimtool.Module + " v0.0.0\nreplace " + gosimtool.Module + " => " + strconv.Quote(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	var patch string
	for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		name := strings.Join(features, "_")
		sub := filepath.Join(dir, name)
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		s := selection(t, features...)
		patch = s.Identity.Toolchain
		if _, err := Generate(s, sub); err != nil {
			t.Fatal(err)
		}
		// Native probes deliberately lack translated machine globals. Serialize
		// only their shared process setup; candidate tests execute the unchanged
		// generated source with real isolation and no such fixture adaptation.
		path := filepath.Join(sub, "composition.go")
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Replace(string(source), "import (", "import (\n \"sync\"", 1)
		text = strings.Replace(text, "processhooks.ConfigureProcess(", "nativeConfigureProcess(", 1)
		text += `
var nativeProcessMutex sync.Mutex
func nativeConfigureProcess(args,env []string) error {
 nativeProcessMutex.Lock()
 defer nativeProcessMutex.Unlock()
 return processhooks.ConfigureProcess(args,env)
}
`
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		test := strings.ReplaceAll(driverProbe, "FEATURES", `"`+strings.Join(features, `","`)+`"`)
		if err := os.WriteFile(filepath.Join(sub, "driver_test.go"), []byte(test), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	wrong := selection(t, "rt")
	if wrong.Identity.HostOS == "darwin" {
		wrong.Identity.HostOS = "linux"
	} else {
		wrong.Identity.HostOS = "darwin"
		wrong.Identity.HostArch = "arm64"
		wrong.Identity.SimulationArch = "arm64"
	}
	// The architecture-specific hook digest must remain canonical when this
	// host-negative fixture changes architecture on an amd64 worker.
	if wrong.Identity.HostArch == runtime.GOARCH {
		mismatch := filepath.Join(dir, "wrong_host")
		if err := os.Mkdir(mismatch, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Generate(wrong, mismatch); err != nil {
			t.Fatal(err)
		}
		probe := `package gosimcomposition
import("strings";"testing";simrt "github.com/neria-cloud/gosim/rt/runtime")
func TestWrongHost(t *testing.T){called:=false;r:=Driver{}.Run(simrt.RunConfig{},func(){called=true});if called||!r.Failed||!strings.Contains(r.Err,"native runtime identity"){t.Fatal(r)}}`
		if err := os.WriteFile(filepath.Join(mismatch, "driver_test.go"), []byte(probe), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tc, err := buildcfg.Resolve(patch)
	if err != nil {
		t.Fatal(err)
	}

	for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		cmd := buildcfg.Command(tc, dir, "list", "-mod=mod", "-deps", "./"+strings.Join(features, "_"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("list generated graph: %v\n%s", err, out)
		}
		for _, path := range strings.Fields(string(out)) {
			for _, feature := range []string{"fs", "net"} {
				provider := gosimtool.Module + "/internal/" + feature
				if (path == provider || strings.HasPrefix(path, provider+"/")) && !slices.Contains(features, feature) {
					t.Fatalf("disabled provider in graph: %s", path)
				}
			}
		}
	}
	args := []string{"test", "-mod=mod", "-ldflags=-checklinkname=0", "-tags=linkname", "-count=1", "./..."}
	if isRaceBuild() {
		args = append(args[:1], append([]string{"-race"}, args[1:]...)...)
	}
	cmd := buildcfg.Command(tc, dir, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated native Driver %s: %v\n%s", patch, err, out)
	}
}

const driverProbe = `package gosimcomposition
import (
 "bytes"
 "slices"
 "strings"
 "sync/atomic"
 "testing"
 "github.com/neria-cloud/gosim/internal/contracts"
 "github.com/neria-cloud/gosim/internal/control"
 "github.com/neria-cloud/gosim/internal/kernel"
 simrt "github.com/neria-cloud/gosim/rt/runtime"
)
var initCount atomic.Int32
func init(){
 simrt.RegisterPackage("probe").Globals=&simrt.Globals{Globals:&simrt.Global[struct{}]{},Initializers:func(shared bool){
  if shared{panic("unexpected shared pass")}
  if control.CurrentRef().ID==0 {panic("application init preceded control binding/root publication")}
  initCount.Add(1)
 }}
}
func submit(inv kernel.Invocation) kernel.Result {
 if err:=control.Submit(inv.Route,&inv);err!=nil {panic(err)}
 return inv.TakeResult()
}
func TestDriver(t *testing.T){
 want:=[]string{FEATURES}
 main:=func(){
  info,err:=kernel.DecodeInfoResult(submit(kernel.InfoRequest{}.Invocation()));if err!=nil{panic(err)}
  if !slices.Equal(info.Features,want)||info.BuildDigest!=IdentityDigest||info.Certified||info.Seed!=77 {panic("wrong kernel identity")}
  if !slices.Contains(simrt.Envs(),"PROBE=value"){panic("missing runner environment")}
  if slices.Contains(want,"fs") {
   opened,err:=kernel.DecodeOpenatResult(submit(kernel.OpenatRequest{DirFD:-100,Path:"/",Flags:0}.Invocation()));if err!=nil{panic(err)}
   if _,err:=kernel.DecodeCloseResult(submit(kernel.CloseRequest{FD:opened.FD}.Invocation()));err!=nil{panic(err)}
  }
  if slices.Contains(want,"net") {
   socket,err:=kernel.DecodeSocketResult(submit(kernel.SocketRequest{Domain:2,Type:1|0x800|0x80000}.Invocation()));if err!=nil{panic(err)}
   if _,err:=kernel.DecodeCloseResult(submit(kernel.CloseRequest{FD:socket.FD}.Invocation()));err!=nil{panic(err)}
  }
  simrt.WriteLog([]byte("generated driver passed\n"))
 }
 cfg:=simrt.RunConfig{Seed:77,ExtraEnv:[]string{"PROBE=value"}}
 first:=Driver{}.Run(cfg,main)
 second:=Driver{}.Run(cfg,main)
 for _,result:=range []simrt.RunResult{first,second}{
  if result.Failed||result.Err!=""||result.BuildDigest!=IdentityDigest||result.ReplayRevision!=simrt.ReplayRevision||len(result.Checksum)==0||!bytes.Contains(result.LogOutput,[]byte("generated driver passed")){t.Fatalf("bad run: err=%s failed=%t log=%s",result.Err,result.Failed,result.LogOutput)}
 }
 if initCount.Load()!=2{t.Fatal("wrong init count",initCount.Load())}
 if !bytes.Equal(first.Checksum,second.Checksum){t.Fatal("replay differs")}
 if control.CurrentRef().ID!=0{t.Fatal("control binding survived cleanup")}
 for _,feature:=range []struct{name string;flag contracts.Feature}{{"fs",contracts.FS},{"net",contracts.Net}} {
  if slices.Contains(want,feature.name){continue}
  returned,deferred:=false,false
  result:=Driver{}.Run(cfg,func(){defer func(){recover();deferred=true}();submit(kernel.RequireRequest{Feature:feature.flag,Operation:"probe"}.Invocation());returned=true})
  if !result.Failed||!strings.Contains(result.Err,"FeatureDisabled")||returned||deferred{t.Fatalf("disabled feature continued: %+v %t %t",result,returned,deferred)}
 }
 result:=Driver{}.Run(cfg,func(){simrt.SetAbortError(simrt.ErrTestFailed)})
 if !result.Failed{t.Fatal("failed application reported success")}
 if result:=(Driver{}).Run(cfg,main);result.Failed{t.Fatalf("failed run leaked state: %+v",result)}
 if result:= (Driver{}).Run(simrt.RunConfig{Seed:77,Simtrace:"unknown"},main);!result.Failed||!strings.Contains(result.Err,"unknown traceflag"){t.Fatalf("bad trace accepted: %+v",result)}
}
`
