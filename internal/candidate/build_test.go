package candidate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/hostabi"
	suites "github.com/neria-cloud/gosim/internal/tests/features"
	"github.com/neria-cloud/gosim/internal/translate"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func TestCandidate(t *testing.T) {
	if os.Getenv("GOSIM_TEST_LARGE") != "1" {
		t.Skip("run with make test-large")
	}

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// Each job exercises its selected compiler without installing other patches.
	for _, patch := range []string{runtime.Version()} {
		t.Run(patch, func(t *testing.T) {
			tc, err := buildcfg.Resolve(patch)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "./internal/candidate", "-timeout=25m", "-run=^TestCandidateWorker$", "-count=1", "-v"}
			if hostabi.RaceEnabled {
				args = append(args, "-race")
			}
			cmd := buildcfg.Command(tc, root, args...)
			cmd.Env = append(cmd.Env, "GOSIM_CANDIDATE_WORKER=1")
			var output bytes.Buffer
			cmd.Stdout = io.MultiWriter(&output, os.Stdout)
			cmd.Stderr = cmd.Stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("candidate worker %s: %v\n%s", patch, err, output.Bytes())
			}
		})
	}
}

func TestCandidateWorker(t *testing.T) {
	if os.Getenv("GOSIM_TEST_LARGE") != "1" {
		t.Skip("run with make test-large")
	}

	if os.Getenv("GOSIM_CANDIDATE_WORKER") != "1" {
		return
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	tuple := os.Getenv("GOSIM_LARGE_TUPLE")
	if tuple == "" || tuple == "rt,fs,net" {
		t.Run("plaintext-grpc", func(t *testing.T) {
			cfg := buildspec.Config{Schema: 2, Features: []string{"rt", "fs", "net"}, SimulationOS: "linux"}
			_, err := Build(cfg, buildspec.BuildOptions{}, []string{"./internal/tests/net"})
			if err != nil {
				t.Fatalf("build translated plaintext gRPC tests: %v", err)
			}
		})
	}
	project := t.TempDir()
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	mod := "module " + gosimtool.Module + "/.gosim/candidateprobe\n\ngo 1.25.0\nrequire " + gosimtool.Module + " v0.0.0\nreplace " + gosimtool.Module + " => " + strconv.Quote(root) + "\n"
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "payload.txt"), []byte("embedded"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.sum"), sum, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "probe_test.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}
	tc, err := buildcfg.Resolve(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	tidy := buildcfg.Command(tc, project, "mod", "tidy")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("fixture module: %v\n%s", err, out)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	featureSets := [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}}
	if tuple != "" {
		selected, err := buildspec.Features(strings.Split(tuple, ","))
		if err != nil {
			t.Fatal(err)
		}
		featureSets = [][]string{selected}
	}
	for _, features := range featureSets {
		t.Run(strings.Join(features, ","), func(t *testing.T) {
			cfg := buildspec.Config{Schema: 2, Features: features, SimulationOS: "linux"}
			if err := os.Chdir(root); err != nil {
				t.Fatal(err)
			}
			// A prepared tuple runs only its additional candidate coverage.
			if tuple == "" {
				packages, err := suites.Packages(features)
				if err != nil {
					t.Fatal(err)
				}
				positive, err := Build(cfg, buildspec.BuildOptions{Race: hostabi.RaceEnabled}, packages)
				if err != nil {
					t.Fatal(err)
				}
				if len(positive.Artifacts) != len(packages) {
					t.Fatal("missing applicable package artifacts")
				}
				for _, artifact := range positive.Artifacts {
					cmd := exec.Command(filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path), "-test.v")
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("positive suite %s: %v\n%s", artifact.Manifest.PackageDigests[0].Path, err, out)
					}
				}
			}
			checkDisabledFixtures(t, cfg)
			if len(features) == 3 {
				checkTranslatedFaults(t, cfg)
			}
			if err := os.Chdir(project); err != nil {
				t.Fatal(err)
			}
			built, err := Build(cfg, buildspec.BuildOptions{Race: hostabi.RaceEnabled}, []string{"."})
			if err != nil {
				t.Fatal(err)
			}
			if len(built.Artifacts) != 1 {
				t.Fatal("artifact count", len(built.Artifacts))
			}
			if reportDir := os.Getenv("GOSIM_CANDIDATE_REPORT_DIR"); reportDir != "" {
				data, err := json.Marshal(built.Artifacts)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(reportDir, "build.log"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			artifact := built.Artifacts[0]
			checkArtifactIdentitySeparation(t, artifact)
			current, err := translate.InspectDependencies(&translate.TranslateInput{Packages: []string{"."}, Cfg: built.Config})
			if err != nil {
				t.Fatal(err)
			}
			pkg := artifact.Manifest.PackageDigests[0].Path
			found, err := gosimtool.LookupArtifact(project, built.Config.Identity, pkg, current[pkg])
			if err != nil || found.Directory != artifact.Directory {
				t.Fatal("real prebuilt discovery disagrees with build", found.Directory, err)
			}
			for _, feature := range []string{"fs", "net"} {
				found := false
				for _, deps := range built.Dependencies {
					for path := range deps {
						if strings.HasPrefix(path, filepath.Join(root, "internal", feature)+string(os.PathSeparator)) {
							found = true
						}
					}
				}
				if found != slices.Contains(features, feature) {
					t.Fatalf("original graph provider %s presence=%v", feature, found)
				}
			}
			before, err := os.Stat(filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path))
			if err != nil {
				t.Fatal(err)
			}
			again, err := Build(cfg, buildspec.BuildOptions{Race: hostabi.RaceEnabled}, []string{"."})
			if err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(filepath.Join(again.Artifacts[0].Directory, again.Artifacts[0].Manifest.Binary.Path))
			if err != nil {
				t.Fatal(err)
			}
			if artifact.Directory != again.Artifacts[0].Directory || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("cache hit rebuilt artifact: identity %s -> %s, path %s -> %s", built.Config.Digest, again.Config.Digest, artifact.Directory, again.Artifacts[0].Directory)
			}
			configs := []simrt.RunConfig{{Test: "TestRuntime", Seed: 42, ExtraEnv: []string{"CANDIDATE_ENV=owned"}}, {Test: "TestRuntime", Seed: 42, ExtraEnv: []string{"CANDIDATE_ENV=owned"}}}
			for _, feature := range []string{"fs", "net"} {
				if !slices.Contains(features, feature) {
					name := "TestDisabledFS"
					if feature == "net" {
						name = "TestDisabledNet"
					}
					configs = append(configs, simrt.RunConfig{Test: name, Seed: 42, ExtraEnv: []string{"CANDIDATE_ENV=owned"}}, simrt.RunConfig{Test: name + "Control", Seed: 42})
				}
			}
			configs = append(configs, simrt.RunConfig{Test: "TestInitRoot", Seed: 42, ExtraEnv: []string{"INIT_PANIC=root"}}, simrt.RunConfig{Test: "TestInitChild", Seed: 42, ExtraEnv: []string{"INIT_PANIC=child"}})
			configs = append(configs, simrt.RunConfig{Test: "TestSelfTransitions", Seed: 42}, simrt.RunConfig{Test: "TestRootExit", Seed: 42}, simrt.RunConfig{Test: "TestRootCrash", Seed: 42}, simrt.RunConfig{Test: "TestFailure", Seed: 42})
			configs = append(configs, simrt.RunConfig{Test: "TestRuntime", Seed: 42, ExtraEnv: []string{"CANDIDATE_ENV=owned"}})
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			results, err := runRequests(ctx, artifact, configs)
			if err != nil {
				t.Fatal(err)
			}
			for i, result := range results {
				if configs[i].Test == "TestRuntime" {
					if result.Failed {
						t.Fatalf("translated runtime failed: %s\n%s", result.Err, result.LogOutput)
					}
					if !bytes.Contains(result.LogOutput, []byte(`"owned":true`)) {
						t.Fatal("structured log fields lost", string(result.LogOutput))
					}
					if len(result.Checksum) == 0 || !bytes.Equal(results[0].Checksum, result.Checksum) {
						t.Fatal("same-process replay differs", i)
					}
				} else if configs[i].Test == "TestSelfTransitions" || configs[i].Test == "TestRootExit" || configs[i].Test == "TestRootCrash" {
					if result.Failed {
						t.Fatalf("self transition: %s\n%s", result.Err, result.LogOutput)
					}
				} else if strings.HasPrefix(configs[i].Test, "TestInit") {
					if !result.Failed || !bytes.Contains(result.LogOutput, []byte("injected init failure")) || bytes.Contains(result.LogOutput, []byte("init failure ran")) || bytes.Contains(result.LogOutput, []byte("init failure returned")) {
						t.Fatalf("init failure not fatal: %+v", result)
					}
				} else if configs[i].Test == "TestFailure" {
					if !result.Failed || result.Err != "test failed" || !bytes.Contains(result.LogOutput, []byte("expected failure")) {
						t.Fatalf("failure reporting lost: %+v", result)
					}
				} else if !result.Failed || !strings.Contains(result.Err, "FeatureDisabled") || strings.Contains(result.Err, "unwound") || strings.Contains(result.Err, "returned") {
					t.Fatalf("disabled feature did not abort: %+v", result)
				}
			}
			replay, err := Run(ctx, artifact, configs[0])
			if err != nil || replay.Failed || !bytes.Equal(results[0].Checksum, replay.Checksum) {
				t.Fatal("cross-process replay differs", err, replay.Err)
			}
			traced := configs[0]
			traced.Simtrace = "network,syscall"
			traceResults, err := runRequests(ctx, artifact, []simrt.RunConfig{traced, traced})
			if err != nil {
				t.Fatal(err)
			}
			if traceResults[0].Failed || traceResults[1].Failed || !bytes.Equal(traceResults[0].Checksum, traceResults[1].Checksum) {
				t.Fatal("traced replay failed", traceResults)
			}
			if !bytes.Contains(traceResults[0].LogOutput, []byte("call ")) {
				t.Fatal("missing syscall trace")
			}
			if slices.Contains(features, "net") && !bytes.Contains(traceResults[0].LogOutput, []byte(`"traceKind":"network"`)) {
				t.Fatal("missing network trace")
			}
			forged := artifact
			forged.Manifest.Identity.Toolchain = "go1.26.999"
			if _, err := Run(ctx, forged, configs[0]); err == nil {
				t.Fatal("runner accepted another compiler identity")
			}
		})
	}
}

const probe = `package probe
import (
 _ "embed"
 "context"
 "hash/maphash"
 "log/slog"
 "golang.org/x/sys/unix"
 "errors"
 "os"
 "net"
 "io"
 "testing"
 "time"
 "github.com/neria-cloud/gosim"
 simfs "github.com/neria-cloud/gosim/fs"
 simnet "github.com/neria-cloud/gosim/net"
 "github.com/neria-cloud/gosim/rt"
 "github.com/neria-cloud/gosim/internal/control"
 "github.com/neria-cloud/gosim/internal/contracts"
 "github.com/neria-cloud/gosim/internal/kernel"
)
//go:embed payload.txt
var embeddedText string
//go:embed payload.txt
var embeddedBytes []byte
var derived=string(embeddedBytes)
var values=map[string]int{"answer":42}
var array [4]int
const arrayLength = len(array)
var initialized bool
func init(){
 if os.Getenv("INIT_PANIC")=="root"||os.Getenv("INIT_PANIC")=="child"&&gosim.CurrentMachine().ID()>1 {panic("injected init failure")}
 if !gosim.IsSim()||gosim.CurrentMachine().ID()==0{panic("application initialized before root/control binding")}
 if embeddedText!="embedded"||string(embeddedBytes)!="embedded"||derived!="embedded"{panic("embedded globals not initialized before expressions/init")}
 initialized=true
}
func submit(inv kernel.Invocation) kernel.Result {
 if err:=control.Submit(inv.Route,&inv);err!=nil{panic(err)}
 return inv.TakeResult()
}
func TestInitRoot(t *testing.T){panic("init failure returned to main")}
func TestInitChild(t *testing.T){
 child,err:=gosim.NewMachine(gosim.MachineConfig{Main:func(){panic("init failure ran child main")}});if err!=nil{t.Fatal(err)}
 if err:=child.Start();err!=nil{t.Fatal(err)};_,_=child.Wait()
}
func TestFailure(t *testing.T){t.Error("expected failure")}
func TestRuntime(t *testing.T){
 t.Log("candidate diagnostic")
 seed:=maphash.MakeSeed()
 if maphash.String(seed,"value")!=maphash.Bytes(seed,[]byte("value")){t.Fatal("hash representations differ")}
 var random [8]byte
 if n,err:=unix.Getrandom(random[:],0);n!=len(random)||err!=nil{t.Fatal("x/sys randomness",n,err)}
 slog.Info("candidate structured", "owned", true, "hash",maphash.String(seed,"value"),"random",random)

 if os.Getenv("CANDIDATE_ENV")!="owned" {t.Fatal("runner environment did not reach process initialization")}
 if !initialized||arrayLength!=4||cap(array)!=4{t.Fatal("init or array constants failed")}
 if context.Canceled==nil{t.Fatal("error sentinel not initialized")}
 inv:=kernel.StateRequest{Machine:contracts.MachineRef{RunID:999,ID:1}}.Invocation()
 wrongRun:=control.Submit(inv.Route,&inv)
 if !errors.Is(wrongRun,contracts.ErrWrongRun){t.Fatal("cross-machine domain error identity",wrongRun)}
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 if !errors.Is(ctx.Err(),context.Canceled){t.Fatal("cancellation lost error identity")}
 ch:=make(chan int,1)
 go func(){ch<-values["answer"]}()
 if <-ch!=42{t.Fatal("scheduler/channel/map failed")}
 before:=time.Now();time.Sleep(5*time.Nanosecond)
 if time.Since(before)!=5*time.Nanosecond{t.Fatal("simulated time failed")}
 embeddedBytes[0]='X'
 values["answer"]=99
 info:=rt.Info()
 if info.Certified||info.BuildDigest==""||info.ReplayRevision==""{t.Fatal("candidate run metadata",info)}
 changedInfo:=rt.Info();changedInfo.Features[0]="changed"
 if rt.Info().Features[0]!="rt"{t.Fatal("Info aliases run metadata")}
 if err:=rt.SetTimeout(time.Minute);err!=nil{t.Fatal(err)}
 if err:=rt.SetTimeout(0);!errors.Is(err,gosim.ErrInvalidConfig){t.Fatal(err)}
 child,err:=gosim.NewMachine(gosim.MachineConfig{Label:"child",Main:func(){
  if !initialized||values["answer"]!=42||os.Getenv("CANDIDATE_ENV")!="owned"{panic("child initialization or isolation failed")}
  values["answer"]=7
 }});if err!=nil{t.Fatal(err)}
 if state,err:=child.State();err!=nil||state!=gosim.New{t.Fatal("new machine started",state,err)}
 if label,err:=child.Label();err!=nil||label!="child"{t.Fatal(label,err)}
 if child.ID()==0||child.RunID()!=gosim.CurrentMachine().RunID(){t.Fatal("machine identity")}
 if _,err:=child.Wait();!errors.Is(err,gosim.ErrNotStarted){t.Fatal("wait before start",err)}
 for _,feature:=range info.Features{
  switch feature{
  case "fs":
   if err:=simfs.ConfigureMachine(child,simfs.DefaultConfig());err!=nil{t.Fatal(err)}
   if err:=simfs.SetCrashMode(child,simfs.SamplePending);err!=nil{t.Fatal(err)}
   if err:=simfs.SetSyncFault(child,simfs.SyncFault{});err!=nil{t.Fatal(err)}
  case "net":
   if err:=simnet.ConfigureMachine(child,simnet.MachineConfig{});err!=nil{t.Fatal(err)}
   if _,err:=simnet.Address(child);!errors.Is(err,gosim.ErrNotStarted){t.Fatal("address before allocation",err)}
  }
 }
 if err:=child.Start();err!=nil{t.Fatal(err)}
 exit,err:=child.Wait();if err!=nil||exit.Kind!=gosim.Graceful||exit.Epoch==0{t.Fatal(exit,err)}
 if err:=child.Restart();err!=nil{t.Fatal(err)}
 next,err:=child.Wait();if err!=nil||next.Kind!=gosim.Graceful||next.Epoch<=exit.Epoch{t.Fatal(next,err)}
 if string(embeddedBytes)!="Xmbedded"{t.Fatal("child changed embedded bytes")}
 if values["answer"]!=99{t.Fatal("child changed root globals")}
 if err:=child.Crash();err!=nil{t.Fatal(err)}
 if terminal,err:=child.Wait();err!=nil||terminal!=next{t.Fatal("terminal exit changed",terminal,err)}
 if err:=child.Destroy();err!=nil{t.Fatal(err)}
 if _,err:=child.State();!errors.Is(err,gosim.ErrMachineNotFound){t.Fatal("destroyed handle",err)}
 for _,feature:=range info.Features{
  switch feature{
  case "fs":
   if err:=os.WriteFile("/probe",[]byte("content"),0600);err!=nil{t.Fatal(err)}
   entries,err:=os.ReadDir("/");if err!=nil||len(entries)==0{t.Fatal("directory open/read",err)}
   data,err:=os.ReadFile("/probe");if err!=nil||string(data)!="content"{t.Fatal("filesystem",err,string(data))}
  case "net":
   socket,err:=kernel.DecodeSocketResult(submit(kernel.SocketRequest{Domain:2,Type:1|0x800|0x80000}.Invocation()));if err!=nil{t.Fatal(err)}
   if _,err:=kernel.DecodeCloseResult(submit(kernel.CloseRequest{FD:socket.FD}.Invocation()));err!=nil{t.Fatal(err)}
   address,err:=simnet.Address(gosim.CurrentMachine());if err!=nil{t.Fatal(err)}
   listener,err:=net.Listen("tcp4",address.String()+":8080");if err!=nil{t.Fatal(err)}
   done:=make(chan error,1)
   go func(){conn,err:=listener.Accept();if err!=nil{done<-err;return};defer conn.Close();_,err=io.CopyN(conn,conn,2);done<-err}()
   conn,err:=net.DialTimeout("tcp4",listener.Addr().String(),time.Second);if err!=nil{t.Fatal(err)}
   if _,err:=conn.Write([]byte("ok"));err!=nil{t.Fatal(err)}
   var data [2]byte;if _,err:=io.ReadFull(conn,data[:]);err!=nil||string(data[:])!="ok"{t.Fatal("tcp echo",err)}
   if err:=conn.Close();err!=nil{t.Fatal(err)}
   if err:=<-done;err!=nil{t.Fatal(err)}
   if err:=listener.Close();err!=nil{t.Fatal(err)}
   ready:=make(chan string,1)
   serving,err:=kernel.DecodeCreateResult(submit(kernel.CreateRequest{Label:"serving",Main:func(){
    address,err:=kernel.DecodeNetAddressResult(submit(kernel.NetAddressRequest{Machine:control.CurrentRef()}.Invocation()));if err!=nil{panic(err)}
    l,err:=net.Listen("tcp4",net.IP(address.Address[:]).String()+":8081");if err!=nil{panic(err)}
    ready<-l.Addr().String()
    c,err:=l.Accept();if err!=nil{panic(err)}
    if _,err:=io.CopyN(c,c,2);err!=nil{panic(err)}
    select{}
   }}.Invocation()));if err!=nil{t.Fatal(err)}
   if _,err:=kernel.DecodeStartResult(submit(kernel.StartRequest{Machine:serving.Machine}.Invocation()));err!=nil{t.Fatal(err)}
   live,err:=net.DialTimeout("tcp4",<-ready,time.Second);if err!=nil{t.Fatal(err)}
   if _,err:=live.Write([]byte("ok"));err!=nil{t.Fatal(err)}
   if _,err:=io.ReadFull(live,data[:]);err!=nil{t.Fatal(err)}
   // Root return must gracefully release live streams while destinations survive.

  }
 }
}
func TestRootExit(t *testing.T){defer func(){panic("root exit unwound")}();gosim.ExitSelf();panic("root exit returned")}
func TestRootCrash(t *testing.T){defer func(){panic("root crash unwound")}();gosim.CrashSelf();panic("root crash returned")}

func TestSelfTransitions(t *testing.T) {
 root:=gosim.CurrentMachine()
 if err:=root.Stop();!errors.Is(err,gosim.ErrSelfTransition){t.Fatal(err)}
 if err:=root.Crash();!errors.Is(err,gosim.ErrSelfTransition){t.Fatal(err)}
 if _,err:=root.Wait();!errors.Is(err,gosim.ErrSelfTransition){t.Fatal(err)}
 if err:=root.Destroy();!errors.Is(err,gosim.ErrSelfTransition){t.Fatal(err)}
 for i,exit:=range []func(){gosim.ExitSelf,gosim.CrashSelf} {
  child,err:=gosim.NewMachine(gosim.MachineConfig{Main:func(){defer func(){panic("self exit unwound")}();exit();panic("self exit returned")}})
  if err!=nil{t.Fatal(err)}
  if err:=child.Start();err!=nil{t.Fatal(err)}
  result,err:=child.Wait();if err!=nil||result.Kind!=gosim.ExitKind(i){t.Fatal(result,err)}
  if err:=child.Destroy();err!=nil{t.Fatal(err)}
 }
}
func TestDisabledFSControl(t *testing.T){
 defer func(){recover();panic("disabled control unwound")}()
 var iter *simfs.CrashIterator
 iter.Next()
 panic("disabled control returned")
}
func TestDisabledNetControl(t *testing.T){
 defer func(){recover();panic("disabled control unwound")}()
 simnet.SetLinkPolicy(simnet.Link{},simnet.LinkPolicy{})
 panic("disabled control returned")
}

func TestDisabledFS(t *testing.T){
 defer func(){recover();panic("disabled filesystem unwound application defers")}()
 os.ReadFile("/disabled")
 panic("disabled filesystem returned")
}
func TestDisabledNet(t *testing.T){
 defer func(){recover();panic("disabled network unwound application defers")}()
 net.Listen("tcp4","127.0.0.1:8080")
 panic("disabled network returned")
}
`
