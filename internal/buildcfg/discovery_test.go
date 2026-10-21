package buildcfg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/testutil"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func fakeDiscoveryGo(t *testing.T, version string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	observed := filepath.Join(root, "observed")
	work := filepath.Join(root, "caller.work")
	info := map[string]string{"GOVERSION": version, "GOROOT": root, "GOHOSTOS": runtime.GOOS, "GOHOSTARCH": runtime.GOARCH, "GOMOD": filepath.Join(root, "go.mod"), "GOWORK": work}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = version ]; then\n printf '%%s\\n' %s\nelse\n printf '%%s\\n' \"$PWD\" \"$GOTOOLCHAIN\" \"$GOWORK\" > %s\n printf '%%s\\n' %s\nfi\n", shellQuote("go version "+version+" "+runtime.GOOS+"/"+runtime.GOARCH), shellQuote(observed), shellQuote(string(data)))
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return root, observed, work
}

func TestDiscoveryUsesCallerGoContextThenFreezesCompiler(t *testing.T) {
	root, observed, work := fakeDiscoveryGo(t, "go1.25.1")
	t.Setenv("PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOTOOLCHAIN", "auto")
	t.Setenv("GOWORK", work)
	r, err := Discover(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Toolchain.Version != "go1.25.1" || r.Context.GoWork != work || r.Context.Directory != root {
		t.Fatal(r)
	}
	data, err := os.ReadFile(observed)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != root+"\nauto\n"+work+"\n" {
		t.Fatalf("discovery overwrote caller selection: %q", data)
	}
	for name, cmd := range map[string]struct {
		work string
		env  []string
	}{
		"application": {work, buildspec.ApplicationCommand(context.Background(), r, "version").Env},
		"output":      {"off", buildspec.OutputCommand(context.Background(), r.Toolchain, root, "version").Env},
	} {
		values := map[string]string{}
		for _, entry := range cmd.env {
			k, v, _ := strings.Cut(entry, "=")
			values[k] = v
		}
		if values["GOTOOLCHAIN"] != "local" || values["GOROOT"] != root || values["GOWORK"] != cmd.work {
			t.Fatalf("%s not frozen in correct context: %v", name, values)
		}
	}
}

func TestDiscoveryExplicitOverrideAndCancellation(t *testing.T) {
	root, observed, _ := fakeDiscoveryGo(t, "go1.26.1")
	t.Setenv("PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOTOOLCHAIN", "auto")
	t.Setenv("GOROOT", "/not-the-selected-installation")
	r, err := Discover(context.Background(), root, "go1.26.1")
	if err != nil || r.Toolchain.Version != "go1.26.1" {
		t.Fatal(r, err)
	}
	data, err := os.ReadFile(observed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\ngo1.26.1\n") {
		t.Fatalf("override not delegated to Go: %q", data)
	}
	if _, err := Discover(context.Background(), root, "go1.25.1"); err == nil {
		t.Fatal("accepted different explicitly requested compiler")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Discover(ctx, root, ""); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestDiscoveryRejectsUnsupportedFamily(t *testing.T) {
	root, _, _ := fakeDiscoveryGo(t, testutil.UnregisteredGo().Version)
	t.Setenv("PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := Discover(context.Background(), root, ""); err == nil || !strings.Contains(err.Error(), "unsupported Go family") {
		t.Fatal(err)
	}
}
