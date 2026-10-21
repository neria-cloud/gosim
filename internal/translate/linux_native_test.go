package translate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func TestLinuxArm64SourceBuildsForNativeDarwin(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("Darwin arm64 native worker probe")
	}
	version, err := buildspec.ParseGoVersion(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	selected, err := buildspec.LookupFamily(version)
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []struct{ id, patch, bounds string }{{selected.Profile, runtime.Version(), fmt.Sprintf("go%d.%d && !go%d.%d", version.Major, version.Minor, version.Major, version.Minor+1)}} {
		t.Run(family.id, func(t *testing.T) {
			// Use the actual selected Linux layout source, then apply the existing
			// translator's filename/constraint rewriting before invoking a native compiler.
			source, err := os.ReadFile("../compat/hooks/kernel/layouts_arm64.go")
			if err != nil {
				t.Fatal(err)
			}
			source = bytes.Replace(source, []byte("//go:build "+family.bounds+" && arm64"), []byte("//go:build "+family.bounds+" && linux && arm64"), 1)
			f, err := decorator.Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			f.Name = dst.NewIdent("layoutprobe")
			translator := &packageTranslator{}
			dstutil.Apply(f, func(c *dstutil.Cursor) bool {
				if _, ok := c.Node().(*dst.File); ok {
					translator.preApply(c)
				}
				return false
			}, nil)
			var out bytes.Buffer
			if err := decorator.Fprint(&out, f); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "//go:build") {
				t.Fatal("source constraint retained")
			}
			name := renameFile(gosimtool.BuildConfig{Identity: buildspec.Identity{SimulationOS: "linux", SimulationArch: "arm64"}}, "layout_linux_arm64.go")
			if name == "layout_linux_arm64.go" {
				t.Fatal("Linux filename still excluded on Darwin")
			}
			dir := t.TempDir()
			for name, data := range map[string]string{
				"go.mod": "module layoutprobe\n\ngo 1.25\n",
				name:     out.String(),
				"layout_test.go": `package layoutprobe
import("runtime";"testing";"unsafe")
func TestNative(t *testing.T){
 if runtime.GOOS!="darwin" || runtime.GOARCH!="arm64"{t.Fatal("not native Darwin")}
 if unsafe.Sizeof(Stat_t{})!=128 || unsafe.Offsetof(Stat_t{}.Mode)!=16 || unsafe.Sizeof(Stat_t{}.Nlink)!=4 || LinuxSYS_FSTATAT!=79{t.Fatal("not Linux arm64 ABI")}
}
`,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			tc, err := buildcfg.Resolve(family.patch)
			if err != nil {
				t.Fatal(err)
			}
			cmd := buildcfg.Command(tc, dir, "test", "-ldflags=-checklinkname=0", "-tags=linkname")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("native build/execution: %v\n%s", err, out)
			}
		})
	}
}
