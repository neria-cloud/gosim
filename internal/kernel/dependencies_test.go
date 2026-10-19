package kernel

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestKernelImportsOnlyContractsAndNativeEngine(t *testing.T) {
	cmd := buildspec.GoCommand(buildspec.Toolchain{Executable: filepath.Join(runtime.GOROOT(), "bin", "go"), GOROOT: runtime.GOROOT(), Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}, "", "list", "-f", "{{join .Imports \"\\n\"}}", "github.com/neria-cloud/gosim/internal/kernel")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range strings.Fields(string(output)) {
		if !strings.HasPrefix(pkg, "github.com/neria-cloud/gosim/") {
			continue
		}
		switch pkg {
		case "github.com/neria-cloud/gosim/internal/contracts", "github.com/neria-cloud/gosim/rt/runtime", "github.com/neria-cloud/gosim/rt/runtime/abi":
		default:
			t.Fatalf("kernel imports implementation/host package %s", pkg)
		}
	}
}
