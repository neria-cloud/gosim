package buildcfg

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestPublishedModuleUsesDiscoveredCache(t *testing.T) {
	root := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), fixtureSharedDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, data, fixtureFilePerm); err != nil {
			t.Fatal(err)
		}
	}
	mod := []byte("module " + GosimModule + "\ngo 1.25\n")
	proxy := filepath.Join(root, "proxy")
	base := filepath.Join(proxy, GosimModule, "@v", "v0.0.1")
	write(base+".mod", mod)
	write(base+".info", []byte(`{"Version":"v0.0.1","Time":"2026-01-01T00:00:00Z"}`))
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entry, err := zw.Create(GosimModule + "@v0.0.1/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(mod); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	write(base+".zip", buf.Bytes())
	app := filepath.Join(root, "app")
	write(filepath.Join(app, "go.mod"), []byte("module example.org/app\ngo 1.25\nrequire "+GosimModule+" v0.0.1\n"))
	cache := filepath.Join(root, "selected-cache")
	t.Cleanup(func() {
		_ = filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				return os.Chmod(path, fixtureSharedDirPerm)
			}
			return err
		})
	})
	envfile := filepath.Join(root, "goenv")
	write(envfile, []byte("GOMODCACHE="+cache+"\n"))
	t.Setenv("GOENV", envfile)
	t.Setenv("GOMODCACHE", "")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOPROXY", (&url.URL{Scheme: "file", Path: proxy}).String())
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "mod", "download", GosimModule)
	cmd.Dir = app
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("download fixture: %v\n%s", err, out)
	}
	r, err := ResolveRunningProcess(t.Context(), app)
	if err != nil {
		t.Fatal(err)
	}
	// GOENV remains disabled in build children; only the resolved cache location
	// is carried forward, even if the caller changes its environment afterward.
	t.Setenv("GOMODCACHE", filepath.Join(root, "other-cache"))
	out, err := buildspec.ApplicationCommand(t.Context(), r, "env", "GOMODCACHE", "GOENV").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != cache+"\n\n" {
		t.Fatalf("lost selected cache: %q", out)
	}
	modules, err := buildspec.ResolveModuleGraph(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := FrameworkModule(modules)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(selected.Effective().Dir, cache+string(filepath.Separator)) || selected.Version != "v0.0.1" {
		t.Fatalf("wrong published source: %+v", selected)
	}
}
