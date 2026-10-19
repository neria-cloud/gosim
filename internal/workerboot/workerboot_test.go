package workerboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestStartupOverrides(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		args            []string
		config, version string
		fail, help      bool
	}{
		{name: "automatic", args: []string{"test", "-race", "./..."}},
		{name: "explicit", args: []string{"test", "-go=go1.26.9", "-config", "some path.json"}, config: "some path.json", version: "go1.26.9"},
		{name: "internal", args: []string{"internal-candidate", "verify", "-ordinary", "-json", "-go", "go1.25.5"}, version: "go1.25.5"},
		{name: "flag value", args: []string{"test", "-run", "-help", "-go", "go1.26.9"}, version: "go1.26.9"},
		{name: "positional", args: []string{"test", "./...", "-go", "go1.26.9"}},
		{name: "separator", args: []string{"test", "--", "-go", "go1.26.9"}},
		{name: "help", args: []string{"test", "-h"}, help: true},
		{name: "missing", args: []string{"test", "-go"}, fail: true},
		{name: "private", args: []string{"test", "--resolution", "file"}, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config, version, err := startupOverrides(tc.args)
			if tc.help {
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatal(err)
				}
				return
			}
			if (err != nil) != tc.fail || config != tc.config || version != tc.version {
				t.Fatalf("got %q %q %v", config, version, err)
			}
		})
	}
}

func TestLauncherHelpDoesNotResolveGo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GOSIM_CONFIG", filepath.Join(t.TempDir(), "missing"))
	for _, args := range [][]string{nil, {"help"}, {"version"}, {"--help"}, {"test", "-h"}, {"internal-candidate", "verify", "--help"}} {
		var out, diagnostics bytes.Buffer
		if code := Main(t.Context(), args, Streams{Out: &out, Err: &diagnostics}); code != 0 || out.Len() == 0 || diagnostics.Len() != 0 {
			t.Fatalf("%v: code=%d stdout=%s stderr=%s", args, code, &out, &diagnostics)
		}
	}
}

func TestWorkerSettings(t *testing.T) {
	t.Parallel()
	base := []debug.BuildSetting{{Key: "GOOS", Value: "darwin"}, {Key: "GOARCH", Value: "arm64"}, {Key: "-tags", Value: "linkname"}, {Key: "-trimpath", Value: "true"}}
	if err := validateSettings(base, "darwin", "arm64", true); err != nil {
		t.Fatal(err)
	}
	for _, setting := range []debug.BuildSetting{{Key: "-ldflags", Value: "-X bad=value"}, {Key: "-race", Value: "true"}, {Key: "-tags", Value: "sim"}, {Key: "GOARCH", Value: "amd64"}, {Key: "-trimpath", Value: "false"}, {Key: "GOEXPERIMENT", Value: "x"}} {
		if err := validateSettings(append(append([]debug.BuildSetting{}, base...), setting), "darwin", "arm64", true); err == nil {
			t.Fatalf("accepted %v", setting)
		}
	}
}

// This minimal fixture exercises real go list/build and native handshakes without
// preparing simulations or sweeping feature/toolchain combinations.
const fixtureWorker = `package main
import (
 "encoding/json"
 "fmt"
 "io"
 "os"
 "runtime"
 "runtime/debug"
 "example.org/worker-dep"
)
func main() {
 if len(os.Args)==2 && os.Args[1]=="--worker-info" {
  b,_:=debug.ReadBuildInfo()
  json.NewEncoder(os.Stdout).Encode(map[string]any{
   "protocol":1, "toolchain":runtime.Version(), "compiler":runtime.Compiler,
   "host_os":runtime.GOOS, "host_arch":runtime.GOARCH, "go_root":runtime.GOROOT(),
   "build_path":b.Path, "module":b.Main, "settings":b.Settings,
  })
  return
 }
 if len(os.Args)<4 || os.Args[1]!="--resolution" { os.Exit(91) }
 fmt.Fprintln(os.Stdout, "invocation="+os.Args[2])
 fmt.Fprintln(os.Stdout, "dependency="+dep.Value)
 io.Copy(os.Stdout,os.Stdin)
 fmt.Fprintln(os.Stderr,"worker diagnostics")
 os.Exit(17)
}
`

func writeFixture(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (buildspec.Resolution, Source, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	framework, app, dep := filepath.Join(root, "framework"), filepath.Join(root, "app"), filepath.Join(root, "dep")
	writeFixture(t, filepath.Join(framework, "go.mod"), "module "+buildcfg.GosimModule+"\n\ngo 1.25\n\nrequire example.org/worker-dep v0.0.0\n")
	writeFixture(t, filepath.Join(framework, "cmd/gosim-worker/main.go"), fixtureWorker)
	writeFixture(t, filepath.Join(dep, "go.mod"), "module example.org/worker-dep\n\ngo 1.25\n")
	writeFixture(t, filepath.Join(dep, "dep.go"), "package dep\nconst Value=\"first\"\n")
	writeFixture(t, filepath.Join(app, "go.mod"), fmt.Sprintf("module example.org/app\n\ngo 1.25\n\nrequire (\n%s v0.0.0\nexample.org/worker-dep v0.0.0\n)\nreplace %s => ../framework\nreplace example.org/worker-dep => ../dep\n", buildcfg.GosimModule, buildcfg.GosimModule))
	writeFixture(t, filepath.Join(app, "app.go"), "package app\n")
	r, err := buildcfg.ResolveRunningProcess(t.Context(), app)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ResolveSource(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if s.Directory != framework {
		t.Fatalf("selected %s, expected %s", s.Directory, framework)
	}
	return r, s, dep
}

func TestWorkerCacheAndForwarding(t *testing.T) {
	t.Parallel()
	r, source, _ := fixture(t)
	inputs, err := buildcfg.Fingerprint(r.Toolchain)
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	var workers [3]Worker
	var failures [3]error
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() { workers[i], failures[i] = Ensure(t.Context(), r, source, inputs, cache) })
	}
	wg.Wait()
	for i := range workers {
		if failures[i] != nil {
			t.Fatal(failures[i])
		}
		if workers[i] != workers[0] {
			t.Fatal("concurrent callers got different workers")
		}
	}
	w := workers[0]
	before, err := os.Stat(w.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(t.Context(), r, source, inputs, cache); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(w.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || before.ModTime() != after.ModTime() {
		t.Fatal("cache hit rebuilt worker")
	}
	var out, diagnostics bytes.Buffer
	code, err := Run(t.Context(), w, r, []string{"test"}, Streams{In: strings.NewReader("caller input\n"), Out: &out, Err: &diagnostics})
	if err != nil || code != 17 || !strings.Contains(out.String(), "caller input") || diagnostics.String() != "worker diagnostics\n" {
		t.Fatalf("forwarding: code=%d err=%v stdout=%s stderr=%s", code, err, &out, &diagnostics)
	}
	invocation := strings.TrimPrefix(strings.Split(out.String(), "\n")[0], "invocation=")
	if _, err := os.Stat(invocation); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invocation file leaked", err)
	}

	// A corrupt binary is replaced on a fresh inode, never executed or truncated.
	if err := os.Remove(w.Path); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, w.Path, "corrupt")
	if _, err := readWorker(filepath.Dir(w.Path), w.Manifest); err == nil {
		t.Fatal("accepted corrupt worker")
	}
	rebuilt, err := Ensure(t.Context(), r, source, inputs, cache)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Manifest != w.Manifest {
		t.Fatal("rebuild changed immutable manifest")
	}
	manifest := filepath.Join(filepath.Dir(w.Path), "manifest.json")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, manifest, string(data)+" {}")
	if _, err := readWorker(filepath.Dir(w.Path), w.Manifest); err == nil {
		t.Fatal("accepted trailing manifest data")
	}
	if _, err := Ensure(t.Context(), r, source, inputs, cache); err != nil {
		t.Fatal(err)
	}
	stages, err := filepath.Glob(filepath.Join(cache, "workers/v1/.worker-*"))
	if err != nil || len(stages) != 0 {
		t.Fatalf("staging leak: %v %v", stages, err)
	}
}

func TestSourceSelectionAndIsolation(t *testing.T) {
	t.Parallel()
	r, source, dep := fixture(t)
	writeFixture(t, filepath.Join(r.Context.Directory, "app.go"), "package app\nconst Changed=1\n")
	unchanged, err := ResolveSource(t.Context(), r)
	if err != nil || unchanged != source {
		t.Fatalf("unrelated caller edit invalidated worker: %v", err)
	}
	name := filepath.Join(dep, "dep.go")
	stat, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, name, "package dep\nconst Value=\"other\"\n")
	if err := os.Chtimes(name, stat.ModTime(), stat.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed, err := ResolveSource(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if changed.GraphDigest == source.GraphDigest || changed.SourceDigest != source.SourceDigest {
		t.Fatal("dependency edit was not isolated to graph identity")
	}
	work := filepath.Join(filepath.Dir(r.Context.Directory), "go.work")
	writeFixture(t, work, "go 1.25\nuse (\n./app\n./framework\n./dep\n)\nreplace example.org/worker-dep v0.0.0 => ./dep\n")
	// Workspace main modules own selection, so remove overlapping replacements.
	writeFixture(t, filepath.Join(r.Context.Directory, "go.mod"), "module example.org/app\n\ngo 1.25\n")
	r.Context.GoWork = work
	workspace, err := ResolveSource(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Directory != source.Directory {
		t.Fatal("workspace selected wrong Gosim")
	}
	r.Context.GoWork = "off"
	if _, err := ResolveSource(t.Context(), r); err == nil || !strings.Contains(err.Error(), "does not contain") {
		t.Fatal("missing framework silently accepted", err)
	}
}

func TestWorkerLockCancellation(t *testing.T) {
	t.Parallel()
	r, source, _ := fixture(t)
	inputs := buildspec.Fingerprints{ToolchainDigest: buildspec.Hash([]byte("go")), StdlibDigest: buildspec.Hash([]byte("stdlib"))}
	want, err := manifestFor(r, source, inputs)
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	dir := filepath.Join(cache, "workers/v1", want.Key)
	unlock, err := buildspec.LockCache(t.Context(), dir+".lock")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := Ensure(ctx, r, source, inputs, cache); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled wait published an entry", err)
	}
}

func TestInvocationRejectsMalformedFiles(t *testing.T) {
	t.Parallel()
	for _, data := range []string{"{}", `{"schema":1,"unknown":true}`, `{} {}`} {
		name := filepath.Join(t.TempDir(), "invocation.json")
		writeFixture(t, name, data)
		if _, err := ValidateInvocation(t.Context(), name); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	data, err := json.Marshal(Invocation{Schema: 1})
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "invocation.json")
	writeFixture(t, name, string(data))
	link := name + ".link"
	if err := os.Symlink(name, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateInvocation(t.Context(), link); err == nil {
		t.Fatal("accepted symlink")
	}
}
