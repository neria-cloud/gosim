package buildcfg

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestCommandPinsExecutableAndEnvironment(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "go")
	// A tiny subprocess reports the actual received environment and arguments.
	script := "#!/bin/sh\nprintf '%s\\n' \"$GOTOOLCHAIN\" \"$GOROOT\" \"$GOENV\" \"$GOWORK\" \"$GOOS/$GOARCH\" \"$GOFLAGS\" \"$PWD\" \"$@\"\n"
	if err := os.WriteFile(exe, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	tc := buildspec.Toolchain{Executable: exe, GOROOT: root, Version: "go1.26.9", HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	t.Setenv("GOWORK", filepath.Join(root, "untrusted.work"))
	cmd := Command(tc, root, "env", "GOVERSION")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{tc.Version, root, "off", "off", tc.HostOS + "/" + tc.HostArch, "", root, "env", "GOVERSION", ""}, "\n")
	if string(out) != want {
		t.Fatalf("child received %q, want %q", out, want)
	}
}

func TestCommandRejectsUnapprovedEnvironment(t *testing.T) {
	tc := buildspec.Toolchain{Executable: filepath.Join(runtime.GOROOT(), "bin", "go"), GOROOT: runtime.GOROOT(), Version: "go1.26.9", HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	for _, setting := range []struct{ key, value string }{
		{"GOFLAGS", "-tags=go1.27"},
		{"GOEXPERIMENT", "noswissmap"},
		{"GODEBUG", "randseednop=0"},
		{"GOTOOLCHAIN", "auto"},
		{"GOTOOLCHAIN", "go1.26.9+auto"},
		{"GOAMD64", "v3"},
		{"GOARM64", "v9.0"},
		{"GOROOT", "/modified"},
		{"GOOS", "windows"},
		{"GOARCH", "riscv64"},
		{"CGO_CFLAGS", "-DCHANGED"},
	} {
		t.Run(setting.key+setting.value, func(t *testing.T) {
			t.Setenv(setting.key, setting.value)
			if err := Command(tc, "", "version").Run(); err == nil {
				t.Fatalf("accepted %s", setting.key)
			}
		})
	}
}

func TestWorkerMismatchGivesRebuildCommand(t *testing.T) {
	t.Parallel()
	tc := buildspec.Toolchain{Version: "go1.25.14", HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	if runtime.Version() == tc.Version {
		tc.Version = "go1.26.9"
	}
	err := ValidateWorker(tc)
	if err == nil || !strings.Contains(err.Error(), "GOTOOLCHAIN="+tc.Version+" go build -ldflags=-checklinkname=0 -tags=linkname") {
		t.Fatalf("missing actionable mismatch error: %v", err)
	}
	if !strings.HasSuffix(err.Error(), "github.com/neria-cloud/gosim/cmd/gosim") {
		t.Fatalf("rebuild command must also work from an application module: %v", err)
	}
}

func TestPinnedDistributionScope(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"go1.25.14", "go1.26.9"} {
		for _, host := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "arm64"}} {
			r, err := distributionRecord(version, host[0], host[1])
			if err != nil || !buildspec.ValidDigest(r.SHA256) || r.Size <= 0 || r.Filename != version+"."+host[0]+"-"+host[1]+".tar.gz" {
				t.Fatalf("bad trust anchor: %+v, %v", r, err)
			}
		}
	}
	if _, err := Resolve("go1.26.0"); err == nil {
		t.Fatal("development baseline became a supported distribution")
	}
	if _, err := distributionRecord("go1.26.9", "darwin", "amd64"); err == nil {
		t.Fatal("accepted unplanned platform")
	}
}

func TestCommandRejectsUnapprovedFlags(t *testing.T) {
	t.Parallel()
	tc := buildspec.Toolchain{Executable: filepath.Join(runtime.GOROOT(), "bin", "go"), GOROOT: runtime.GOROOT(), Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	for _, flags := range [][]string{
		{"-tags=go1.27"},
		{"-tags", "sim,retired"},
		{"-tags=sim,sim"},
		{"-ldflags=-X=some.variable=value"},
		{"-gcflags=all=-N"},
		{"-overlay=changed.json"},
		{"-toolexec=other"},
		{"-compiler=gccgo"},
	} {
		if cmd := Command(tc, "", append([]string{"build"}, flags...)...); cmd.Err == nil {
			t.Fatalf("accepted flags %v", flags)
		}
	}
	if cmd := Command(tc, "", "test", "-tags=linkname,sim", "-ldflags=-checklinkname=0", "-race"); cmd.Err != nil {
		t.Fatal(cmd.Err)
	}
}
