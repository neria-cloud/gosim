package spec

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestUnregisteredInspectionPreparation(t *testing.T) {
	for _, workspace := range []bool{false, true} {
		t.Run(fmt.Sprint(workspace), func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			write := func(name, body string) {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			write("app/go.mod", "module example.org/app\ngo 1.25\nrequire example.org/dep v1.0.0\nreplace example.org/dep => ../dep\n")
			write("dep/go.mod", "module example.org/dep\ngo 1.25\n")
			write("dep/dep.go", "package dep\nfunc Hook(v int) int { return v }\n")
			write("go.work", "go 1.25\nuse ./app\n")
			write("app/go.sum", "example.org/other v1.0.0 h1:fixture\n")
			app, dep := filepath.Join(root, "app"), filepath.Join(root, "dep")
			work := "off"
			if workspace {
				work = filepath.Join(root, "go.work")
			}
			input := []buildspec.Module{{Path: "example.org/app", Main: true, Dir: app, GoMod: filepath.Join(app, "go.mod")}, {Path: "example.org/dep", Version: "v1.0.0", Replace: &buildspec.Module{Path: dep, Dir: dep, GoMod: filepath.Join(dep, "go.mod")}}}
			output := []buildspec.Module{{Path: "gosim-hook-source", Main: true}, {Path: "example.org/app", Version: "v0.0.0", Replace: &buildspec.Module{Path: app, Dir: app}}, input[1]}
			encode := func(items []buildspec.Module) string {
				var s string
				for _, item := range items {
					b, err := json.Marshal(item)
					if err != nil {
						t.Fatal(err)
					}
					s += string(b) + "\n"
				}
				return s
			}
			write("input.json", encode(input))
			write("output.json", encode(output))
			source, _ := json.Marshal(sourcePackage{ImportPath: "example.org/dep", Dir: dep, GoFiles: []string{"dep.go"}})
			quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
			info, _ := json.Marshal(map[string]string{"GOMOD": filepath.Join(app, "go.mod"), "GOWORK": work})
			script := fmt.Sprintf(`#!/bin/sh
printf '%%s|%%s|%%s|%%s\n' "$PWD" "$GOWORK" "$GOTOOLCHAIN" "$*" >> %s
case "$1" in
 env) if [ "$2" = -json ]; then printf '%%s\n' %s; else printf '%%s\n' %s; fi ;;
 list) case " $* " in
  *" -m "*) if [ "$PWD" = %s ]; then cat %s; else printf '%%s\n' "$PWD" > %s; cat %s; fi ;;
  *) printf '%%s\n' %s ;;
 esac ;;
 *) exit 42 ;;
esac
`, quote(filepath.Join(root, "commands")), quote(string(info)), quote(filepath.Join(app, "go.mod")), quote(app), quote(filepath.Join(root, "input.json")), quote(filepath.Join(root, "private")), quote(filepath.Join(root, "output.json")), quote(string(source)))
			write("bin/go", script)
			r := buildspec.Resolution{Toolchain: buildspec.Toolchain{Version: "go1.27.1", GOROOT: root, Executable: filepath.Join(root, "bin/go"), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}, Context: buildspec.ModuleContext{Directory: app, GoWork: work}}
			if _, _, err := hookInspectionContext(t.Context(), r); err == nil {
				t.Fatal("ordinary preparation admitted future family")
			}
			if _, err := buildspec.ModuleFiles(r, input, "gosim-hook-source"); err == nil {
				t.Fatal("ordinary rendering admitted future")
			}
			if buildspec.SourceInspectionApplicationCommand(t.Context(), r, "build").Err == nil {
				t.Fatal("inspection permits builds")
			}
			before, _ := os.ReadFile(filepath.Join(app, "go.mod"))
			dir, cleanup, err := PrepareSourceInspectionContext(t.Context(), r)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := cleanup(); err != nil {
					t.Error(err)
				}
			})
			mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err != nil || !strings.Contains(string(mod), "go 1.27.1") || !strings.Contains(string(mod), dep) {
				t.Fatal(string(mod), err)
			}
			sum, err := os.ReadFile(filepath.Join(dir, "go.sum"))
			if err != nil || !strings.Contains(string(sum), "h1:fixture") {
				t.Fatal(string(sum), err)
			}
			data, err := buildspec.SourceInspectionCommand(t.Context(), r.Toolchain, dir, "list", "-mod=mod", "-json", "-tags=linkname", "example.org/dep").Output()
			if err != nil {
				t.Fatal(err)
			}
			var pkg sourcePackage
			if err := json.Unmarshal(data, &pkg); err != nil {
				t.Fatal(err)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), filepath.Join(pkg.Dir, pkg.GoFiles[0]), nil, 0); err != nil {
				t.Fatal(err)
			}
			commands, _ := os.ReadFile(filepath.Join(root, "commands"))
			callerFlags := "list -mod=readonly -m -json all"
			if workspace {
				callerFlags = "list -m -json all"
			}
			for _, want := range []string{app + "|" + work + "|local|" + callerFlags, dir + "|off|local|list -mod=readonly -m -json all", dir + "|off|local|list -mod=mod -json -tags=linkname example.org/dep"} {
				if !strings.Contains(string(commands), want) {
					t.Fatalf("missing %s in %s", want, commands)
				}
			}
			after, _ := os.ReadFile(filepath.Join(app, "go.mod"))
			if string(before) != string(after) {
				t.Fatal("caller modified")
			}
			if err := cleanup(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatal("cleanup retained module", err)
			}
			// A private graph failure must remove its temporary directory.
			write("output.json", `{broken`)
			if _, _, err := PrepareSourceInspectionContext(t.Context(), r); err == nil {
				t.Fatal("bad private graph accepted")
			}
			private, _ := os.ReadFile(filepath.Join(root, "private"))
			if _, err := os.Stat(strings.TrimSpace(string(private))); !os.IsNotExist(err) {
				t.Fatal("failure leaked module", err)
			}
			// Cleanup failures are returned, not reported as successful removal.
			write("output.json", encode(output))
			dir, cleanup, err = PrepareSourceInspectionContext(t.Context(), r)
			if err != nil {
				t.Fatal(err)
			}
			if os.Geteuid() != 0 {
				if err := os.Chmod(dir, 0); err != nil {
					t.Fatal(err)
				}
				cleanupErr := cleanup()
				if err := os.Chmod(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if cleanupErr == nil {
					t.Fatal("cleanup error suppressed")
				}
			}
			if err := cleanup(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
