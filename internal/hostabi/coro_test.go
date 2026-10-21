//go:build go1.25

package hostabi

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestCoroYieldFinishAndSkippedDefers(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		var c Coro
		stage := 0
		deferred := false
		c.Start(func() {
			defer func() { deferred = true }()
			stage = 1
			c.Yield()
			if stage != 2 {
				panic("caller state did not cross coroutine yield")
			}
			stage = 3
			c.Finish()
			panic("Finish returned")
		})
		if stage != 1 {
			t.Fatalf("Start stage=%d", stage)
		}
		stage = 2
		c.Next()
		if stage != 3 || deferred {
			t.Fatalf("Finish stage=%d deferred=%v", stage, deferred)
		}
	}
}

func TestUpcallRunsAndResumes(t *testing.T) {
	t.Parallel()
	var c Upcallcoro
	stage := 0
	c.Start(func() {
		c.Upcall(func() { stage++ })
		c.Yield()
		c.Upcall(func() { stage++ })
		c.Finish()
	})
	if stage != 1 {
		t.Fatalf("first upcall stage=%d", stage)
	}
	c.Next()
	if stage != 2 {
		t.Fatalf("second upcall stage=%d", stage)
	}
}

func TestIdentityAndFamilyIsolation(t *testing.T) {
	t.Parallel()
	id := Identity()
	if id.Toolchain != runtime.Version() || id.HostOS != runtime.GOOS || id.HostArch != runtime.GOARCH || !buildspec.ValidDigest(id.SourceDigest) {
		t.Fatalf("identity=%+v", id)
	}
	other := "go125"
	if id.Profile == other {
		other = "go126"
	}
	cmd := buildspec.GoCommand(buildspec.Toolchain{Executable: filepath.Join(runtime.GOROOT(), "bin", "go"), GOROOT: runtime.GOROOT(), Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}, "", "list", "-deps", "github.com/neria-cloud/gosim/rt/runtime")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list: %v: %s", err, data)
	}
	if strings.Contains(string(data), "/internal/compat/"+other+"/host") || strings.Contains(string(data), "/internal/compat/"+id.Profile+"/host") || strings.Contains(string(data), "/internal/coro\n") {
		t.Fatalf("wrong native ABI import: %s", data)
	}
	if !strings.Contains(string(data), "/internal/compat/host") {
		t.Fatalf("selected native ABI missing: %s", data)
	}
}

// Inspect source selection without synthesizing manual release-tag builds.
func TestUnsupportedFamilyGuardSelection(t *testing.T) {
	t.Parallel()
	for _, pkg := range []string{"hostabi", "compat/selected", "compat/host"} {
		dir := filepath.Join("..", pkg)
		guard, err := os.ReadFile(filepath.Join(dir, "unsupported.go"))
		if err != nil || !strings.Contains(string(guard), "GOSIM_UNSUPPORTED_GO_FAMILY") {
			t.Fatal(pkg, err)
		}
		for _, minor := range []int{24, 25, 26, 27} {
			ctx := build.Context{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Compiler: "gc"}
			for release := 1; release <= minor; release++ {
				ctx.ReleaseTags = append(ctx.ReleaseTags, fmt.Sprintf("go1.%d", release))
			}
			matches, err := ctx.MatchFile(dir, "unsupported.go")
			if err != nil || matches != (minor == 24 || minor == 27) {
				t.Fatal(pkg, minor, matches, err)
			}
		}
	}
}
