package candidate

import (
	"bytes"
	"context"
	"io"
	"os"
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
	"github.com/neria-cloud/gosim/internal/race"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func TestCandidate(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []string{"go1.25.14", "go1.26.9"} {
		t.Run(patch, func(t *testing.T) {
			tc, err := buildcfg.Resolve(patch)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "./internal/candidate", "-run=^TestCandidateWorker$", "-count=1", "-v"}
			if race.Enabled {
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
	if os.Getenv("GOSIM_CANDIDATE_WORKER") != "1" {
		return
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
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
	for _, features := range [][]string{{"rt"}, {"rt", "fs"}, {"rt", "net"}, {"rt", "fs", "net"}} {
		t.Run(strings.Join(features, ","), func(t *testing.T) {
			cfg := buildspec.Config{Schema: 1, Toolchain: runtime.Version(), Features: features, SimulationOS: "linux"}
			built, err := Build(cfg, buildspec.BuildOptions{Race: race.Enabled}, []string{"."})
			if err != nil {
				t.Fatal(err)
			}
			if len(built.Artifacts) != 1 {
				t.Fatal("artifact count", len(built.Artifacts))
			}
			artifact := built.Artifacts[0]
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
			again, err := Build(cfg, buildspec.BuildOptions{Race: race.Enabled}, []string{"."})
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
					configs = append(configs, simrt.RunConfig{Test: name, Seed: 42, ExtraEnv: []string{"CANDIDATE_ENV=owned"}})
				}
			}
			configs = append(configs, simrt.RunConfig{Test: "TestFailure", Seed: 42})
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
					if len(result.Checksum) == 0 || !bytes.Equal(results[0].Checksum, result.Checksum) {
						t.Fatal("same-process replay differs", i)
					}
				} else if configs[i].Test == "TestFailure" {
					if !result.Failed || result.Err != "test failed" || !bytes.Contains(result.LogOutput, []byte("expected failure")) {
						t.Fatalf("failure reporting lost: %+v", result)
					}
				} else if !result.Failed || !strings.Contains(result.Err, "FeatureDisabled") || strings.Contains(result.Err, "unwound") || strings.Contains(result.Err, "returned") {
					t.Fatalf("disabled feature did not abort: %+v", result)
				}
			}
			forged := artifact
			forged.Manifest.Identity.Certification = "certified"
			if _, err := Run(ctx, forged, configs[0]); err == nil {
				t.Fatal("candidate runner accepted certified identity")
			}
		})
	}
}

const probe = `package probe
import (
 _ "embed"
 "context"
 "errors"
 "os"
 "net"
 "io"
 "testing"
 "time"
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
 if control.CurrentRef().ID==0{panic("application initialized before root/control binding")}
 if embeddedText!="embedded"||string(embeddedBytes)!="embedded"||derived!="embedded"{panic("embedded globals not initialized before expressions/init")}
 initialized=true
}
func submit(inv kernel.Invocation) kernel.Result {
 if err:=control.Submit(inv.Route,&inv);err!=nil{panic(err)}
 return inv.TakeResult()
}
func TestFailure(t *testing.T){t.Error("expected failure")}
func TestRuntime(t *testing.T){
 t.Log("candidate diagnostic")
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
 child,err:=kernel.DecodeCreateResult(submit(kernel.CreateRequest{Label:"child",Main:func(){
  if !initialized||values["answer"]!=42||os.Getenv("CANDIDATE_ENV")!="owned"{panic("child initialization or isolation failed")}
  values["answer"]=7
 }}.Invocation()));if err!=nil{t.Fatal(err)}
 if _,err:=kernel.DecodeStartResult(submit(kernel.StartRequest{Machine:child.Machine}.Invocation()));err!=nil{t.Fatal(err)}
 if _,err:=kernel.DecodeWaitResult(submit(kernel.WaitRequest{Machine:child.Machine}.Invocation()));err!=nil{t.Fatal(err)}
 if _,err:=kernel.DecodeRestartResult(submit(kernel.RestartRequest{Machine:child.Machine}.Invocation()));err!=nil{t.Fatal(err)}
 if _,err:=kernel.DecodeWaitResult(submit(kernel.WaitRequest{Machine:child.Machine}.Invocation()));err!=nil{t.Fatal(err)}
 if string(embeddedBytes)!="Xmbedded"{t.Fatal("child changed embedded bytes")}
 if values["answer"]!=99{t.Fatal("child changed root globals")}
 if _,err:=kernel.DecodeDestroyResult(submit(kernel.DestroyRequest{Machine:child.Machine}.Invocation()));err!=nil{t.Fatal(err)}
 info,err:=kernel.DecodeInfoResult(submit(kernel.InfoRequest{}.Invocation()));if err!=nil{t.Fatal(err)}
 for _,feature:=range info.Features{
  switch feature{
  case "fs":
   if err:=os.WriteFile("/probe",[]byte("content"),0600);err!=nil{t.Fatal(err)}
   data,err:=os.ReadFile("/probe");if err!=nil||string(data)!="content"{t.Fatal("filesystem",err,string(data))}
  case "net":
   socket,err:=kernel.DecodeSocketResult(submit(kernel.SocketRequest{Domain:2,Type:1|0x800|0x80000}.Invocation()));if err!=nil{t.Fatal(err)}
   if _,err:=kernel.DecodeCloseResult(submit(kernel.CloseRequest{FD:socket.FD}.Invocation()));err!=nil{t.Fatal(err)}
   address,err:=kernel.DecodeNetAddressResult(submit(kernel.NetAddressRequest{Machine:control.CurrentRef()}.Invocation()));if err!=nil{t.Fatal(err)}
   listener,err:=net.Listen("tcp4",net.IP(address.Address[:]).String()+":8080");if err!=nil{t.Fatal(err)}
   done:=make(chan error,1)
   go func(){conn,err:=listener.Accept();if err!=nil{done<-err;return};defer conn.Close();var data [2]byte;_,err=io.ReadFull(conn,data[:]);if err==nil{_,err=conn.Write(data[:])};done<-err}()
   conn,err:=net.DialTimeout("tcp4",listener.Addr().String(),time.Second);if err!=nil{t.Fatal(err)}
   if _,err:=conn.Write([]byte("ok"));err!=nil{t.Fatal(err)}
   var data [2]byte;if _,err:=io.ReadFull(conn,data[:]);err!=nil||string(data[:])!="ok"{t.Fatal("tcp echo",err)}
   if err:=conn.Close();err!=nil{t.Fatal(err)}
   if err:=<-done;err!=nil{t.Fatal(err)}
   if err:=listener.Close();err!=nil{t.Fatal(err)}
  }
 }
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
