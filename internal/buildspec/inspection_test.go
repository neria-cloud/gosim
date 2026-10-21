package buildspec

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSourceInspectionCommandBoundary(t *testing.T) {
	tc := Toolchain{Executable: filepath.Join(runtime.GOROOT(), "bin", "go"), GOROOT: runtime.GOROOT(), Version: "go1.27.1", HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	ctx := context.Background()
	for _, args := range [][]string{{"version"}, {"env", "GOMOD"}, {"env", "-json", "GOVERSION", "GOROOT", "GOHOSTOS", "GOHOSTARCH", "GOMOD", "GOWORK", "GOMODCACHE"}, {"list", "-mod=readonly", "-m", "-json", "all"}, {"list", "-json", "-m", "all"}, {"list", "-mod=mod", "-json", "-tags=" + TranslatedSourceTags(true), "-deps", "reflect"}} {
		cmd := SourceInspectionCommand(ctx, tc, ".", args...)
		if cmd.Err != nil {
			t.Fatalf("%v: %v", args, cmd.Err)
		}
		if OutputCommand(ctx, tc, ".", args...).Err == nil {
			t.Fatalf("ordinary command admitted %s", tc.Version)
		}
		for _, required := range []string{"GOTOOLCHAIN=local", "GOWORK=off", "GOENV=off", "GOFLAGS="} {
			if !strings.Contains(strings.Join(cmd.Env, "\n"), required+"\n") {
				t.Fatalf("missing control %s", required)
			}
		}
	}
	for _, args := range [][]string{nil, {"build"}, {"run", "x.go"}, {"test"}, {"env"}, {"env", "-w", "GOTOOLCHAIN=auto"}, {"env", "PATH"}, {"version", "-m", "x"}, {"list", "-json", "-export", "fmt"}, {"list", "-json", "-tags=custom", "fmt"}, {"list", "-json", "-overlay=x", "fmt"}, {"list", "-json", "-toolexec=x", "fmt"}, {"list", "-json", "-compiled", "fmt"}, {"list", "-json", "-m", "-u", "all"}, {"list", "-json", "-m", "-deps", "all"}, {"list", "-json", "fmt", "-mod=mod"}, {"list", "-json", "-json", "fmt"}, {"list", "-json"}} {
		if SourceInspectionCommand(ctx, tc, ".", args...).Err == nil {
			t.Fatalf("admitted %q", args)
		}
	}
	for _, version := range []string{"go1.27rc1", "devel go1.27", "go1.27.01", "go1.27.1-custom"} {
		tc.Version = version
		if SourceInspectionCommand(ctx, tc, ".", "version").Err == nil {
			t.Fatal(version)
		}
	}
	tc.Version = "go1.27.1"
	t.Setenv("GOFLAGS", "-overlay=custom.json")
	if SourceInspectionCommand(ctx, tc, ".", "version").Err == nil {
		t.Fatal("ignored environment")
	}
}
