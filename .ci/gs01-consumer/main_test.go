package main

import (
	"archive/tar"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConstraintRewrite(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"functional", "sim"},
		{"!functional", "!sim"},
		{"(functional && linux) || (!functional && darwin)", "(sim && linux) || (!sim && darwin)"},
		{"functional_extra || functional", "functional_extra || sim"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			source := []byte("//go:build " + tc.in + "\n\n// untouched comment\npackage fixture\n\nfunc TestBody() { /* functional */ }\n")
			got, change, err := rewrite(source, true)
			if err != nil {
				t.Fatal(err)
			}
			if change == nil || change.After != tc.want {
				t.Fatalf("change = %#v", change)
			}
			a, _ := body(source)
			b, _ := body(got)
			if !bytes.Equal(a, b) {
				t.Fatal("body changed")
			}
			if _, _, err := rewrite(source, false); err == nil {
				t.Fatal("accepted production constraint")
			}
		})
	}
	source := []byte("//go:build functional && linux\n// +build functional,linux\n\npackage fixture\n")
	got, _, err := rewrite(source, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("// +build sim,linux")) {
		t.Fatalf("legacy not rewritten: %s", got)
	}
	if _, _, err := rewrite(bytes.ReplaceAll(source, []byte("functional,linux"), []byte("!functional,linux")), true); err == nil {
		t.Fatal("inconsistent legacy accepted")
	}
	unchanged := []byte("//go:build !windows\n\npackage fixture\n// functional in body\n")
	got, change, err := rewrite(unchanged, true)
	if err != nil || change != nil || !bytes.Equal(got, unchanged) {
		t.Fatal("unrelated source changed")
	}
	legacy := []byte("// +build functional linux\n// +build !windows\n\npackage fixture\n")
	if _, change, err := rewrite(legacy, true); err != nil || change == nil {
		t.Fatalf("legacy-only: %v %v", change, err)
	}
}

func TestArchiveSafety(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind byte
	}{
		{"../escape", tar.TypeReg},
		{"/absolute", tar.TypeReg},
		{"a/../../escape", tar.TypeReg},
		{"link", tar.TypeSymlink},
		{"hardlink", tar.TypeLink},
		{"device", tar.TypeChar},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			tw := tar.NewWriter(&b)
			if err := tw.WriteHeader(&tar.Header{Name: tc.name, Typeflag: tc.kind, Mode: 0o600, Linkname: "../outside"}); err != nil {
				t.Fatal(err)
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := extract(&b, t.TempDir()); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for range 2 {
		if err := tw.WriteHeader(&tar.Header{Name: "duplicate", Mode: 0o600, Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extract(&b, t.TempDir()); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func put(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func initGit(t *testing.T, dir string) string {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"}} {
		if _, err := git(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	b, err := git(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func TestPreparationManifestAndIntegrity(t *testing.T) {
	// Synthetic local modules exercise the real Git/object/preparation path without
	// network or external consumer tests. The fake worker has no dependencies.
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOWORK", "off")
	source, framework := t.TempDir(), t.TempDir()
	put(t, source, "go.mod", "module "+consumerModule+"\n\ngo 1.26.0\n")
	put(t, source, "go.sum", "")
	put(t, source, diskPackage+"/disk.go", "package disk\nconst Value = 1\n")
	code := "//go:build functional\n\npackage disk\nimport \"testing\"\n"
	for _, name := range gateA {
		code += "func " + name + "(t *testing.T) { t.Parallel() }\n"
	}
	put(t, source, diskPackage+"/journal_func_test.go", code)
	put(t, source, diskPackage+"/unit_test.go", "//go:build !functional\n\npackage disk\n")
	put(t, source, diskPackage+"/native_linux_test.go", "//go:build functional && linux\n\npackage disk\n")
	revision := initGit(t, source)
	put(t, source, diskPackage+"/disk.go", "dirty worktree must not be copied")
	put(t, framework, "go.mod", "module "+gosimModule+"\n\ngo 1.26.0\n")
	put(t, framework, "cmd/gosim-worker/main.go", "package main\nfunc main() {}\n")
	initGit(t, framework)
	output := filepath.Join(t.TempDir(), "evidence")
	if err := prepare(source, revision, output, framework); err != nil {
		t.Fatal(err)
	}
	m, err := read(output)
	if err != nil {
		t.Fatal(err)
	}
	if m.Revision != revision || m.Repository != consumerModule || m.Original[diskPackage+"/disk.go"] != digest([]byte("package disk\nconst Value = 1\n")) {
		t.Fatal("wrong object identity")
	}
	if !reflect.DeepEqual(m.Selections["native-linux"], m.Selections["sim-linux"]) {
		t.Fatal("selection changed")
	}
	if reflect.DeepEqual(m.Selections["native-linux"], m.Selections["native-darwin"]) {
		t.Fatal("native OS selection was flattened")
	}
	if len(m.Constraints) != 3 || len(m.Added) != 2 || len(m.Commands) != 2 {
		t.Fatalf("incomplete manifest: constraints %d, added %d, commands %d", len(m.Constraints), len(m.Added), len(m.Commands))
	}
	if err := prepare(source, revision, output, framework); err == nil {
		t.Fatal("existing output overwritten")
	}
	if err := verifyOutput(output); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ root, name, content string }{
		{m.Native, diskPackage + "/disk.go", "package disk\n"},
		{m.Sim, diskPackage + "/journal_func_test.go", strings.ReplaceAll(code, "t.Parallel()", "t.Skip()")},
		{m.Sim, diskPackage + "/new_test.go", "package disk\n"},
	} {
		p := filepath.Join(tc.root, tc.name)
		old, readErr := os.ReadFile(p)
		put(t, tc.root, tc.name, tc.content)
		if err := verifyOutput(output); err == nil {
			t.Errorf("mutation accepted: %s", p)
		}
		if readErr == nil {
			put(t, tc.root, tc.name, string(old))
		} else {
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Ensure host Go is available explicitly, rather than treating tool absence as a pass.
	if err := exec.Command("go", "version").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceRequiresExecutionAndPasses(t *testing.T) {
	seeds := []string{"1", "2", "3"}
	var valid strings.Builder
	for _, seed := range seeds {
		for _, name := range gateA {
			valid.WriteString("=== RUN   " + name + " (seed " + seed + ")\n--- PASS: " + name + " (0.00s simulated)\n")
		}
	}
	if err := validateAcceptanceLog([]byte(valid.String()), seeds); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "compile succeeded", "lacks base", strings.Replace(valid.String(), "seed 3", "seed 2", 1), strings.Replace(valid.String(), "--- PASS:", "--- SKIP:", 1), valid.String() + "--- FAIL: subtest\n"} {
		if err := validateAcceptanceLog([]byte(bad), seeds); err == nil {
			t.Fatalf("invalid evidence accepted: %s", bad)
		}
	}
}
