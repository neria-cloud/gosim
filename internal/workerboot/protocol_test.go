package workerboot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestProbeRejectsInvalidWorkers(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		t.Fatal(err)
	}
	r := buildspec.Resolution{Schema: 1, Toolchain: buildspec.Toolchain{Version: runtime.Version(), GOROOT: root, Executable: filepath.Join(root, "bin/go"), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}, Context: buildspec.ModuleContext{Directory: t.TempDir(), GoWork: "off"}}
	valid := Info{Protocol: Protocol, Toolchain: runtime.Version(), Compiler: "gc", HostOS: runtime.GOOS, HostArch: runtime.GOARCH, GOROOT: root, BuildPath: WorkerPackage, Module: debug.Module{Path: buildcfg.GosimModule}}
	for _, tc := range []struct {
		name   string
		change func(*Info)
	}{
		{"protocol", func(i *Info) { i.Protocol++ }},
		{"compiler", func(i *Info) { i.Compiler = "gccgo" }},
		{"patch", func(i *Info) { i.Toolchain = "go1.25.999" }},
		{"architecture", func(i *Info) { i.HostArch = "wrong" }},
		{"source", func(i *Info) { i.Module.Path = "example.org/unrelated" }},
		{"package", func(i *Info) { i.BuildPath = buildcfg.GosimModule + "/cmd/gosim" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			info := valid
			tc.change(&info)
			data, err := json.Marshal(info)
			if err != nil {
				t.Fatal(err)
			}
			worker := script(t, "printf '%s' '"+strings.ReplaceAll(string(data), "'", "'\\''")+"'\n")
			if _, err := Probe(t.Context(), worker, r); err == nil {
				t.Fatal("accepted mismatched worker")
			}
		})
	}
	for _, tc := range []struct{ name, body, want string }{
		{"malformed", "printf '{'", "invalid worker handshake"},
		{"extra field", `printf '{"unknown":true}'`, "invalid worker handshake"},
		{"trailing", "printf '{} {}'", "trailing worker handshake data"},
		{"oversized", "dd if=/dev/zero bs=1024 count=65 2>/dev/null", "output limit"},
		{"failed", "printf failed >&2; exit 7", "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Probe(t.Context(), script(t, tc.body), r); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatal(err)
			}
		})
	}
	t.Run("canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()
		if _, err := Probe(ctx, script(t, "sleep 30"), r); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	})
}

func script(t *testing.T, body string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "worker")
	if err := os.WriteFile(name, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return name
}
