package routegen

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestInventoryCoversLegacyRoutes(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	legacy := map[string]bool{}
	for _, route := range m.Routes {
		for _, old := range route.Legacy {
			legacy[old] = true
		}
	}
	// This source audit remains until the legacy path is deleted in Task 26.
	for _, name := range []string{"linux_gen_amd64.go", "linux_gen_arm64.go", "gosim_gen.go", "os_linux.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("../simulation", name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if iface, ok := n.(*ast.InterfaceType); ok {
				for _, field := range iface.Methods.List {
					for _, name := range field.Names {
						if !legacy[name.Name] {
							t.Errorf("unclassified legacy route %s", name.Name)
						}
					}
				}
			}
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Recv != nil && strings.HasPrefix(fn.Name.Name, "Sys") && !legacy[fn.Name.Name] {
				t.Errorf("unclassified manual syscall %s", fn.Name.Name)
			}
			return true
		})
	}
	// Preserve the published intermediate route numbers when adding controls.
	expected := strings.Fields("Read Write Close FcntlInt Getrandom Getpid Uname PollOpen PollClose PollWait PollReset PollDeadline PollUnblock Wait Openat Renameat Getdents64 Fallocate Seek Pwrite Pread Sync Fstat Statat Unlinkat Truncate Chdir Getcwd Mkdirat Flock FcntlLock Map Unmap Advise Socket Bind Listen Accept Connect LocalAddress PeerAddress SetOption GetOption")
	for i, name := range expected {
		if m.Routes[i].Name != name || m.Routes[i].ID != uint32(i+1) {
			t.Fatalf("stable route %d changed", i+1)
		}
	}
	for _, iface := range []string{"../contracts/control_fs.go", "../contracts/control_net.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), iface, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		prefix := "FS"
		if strings.HasSuffix(iface, "control_net.go") {
			prefix = "Net"
		}
		names := map[string]bool{}
		for _, r := range m.Routes {
			names[r.Name] = true
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if typ, ok := n.(*ast.InterfaceType); ok {
				for _, field := range typ.Methods.List {
					for _, name := range field.Names {
						if !names[prefix+name.Name] {
							t.Errorf("missing control %s%s", prefix, name.Name)
						}
					}
				}
			}
			return true
		})
	}
}

func TestInvalidRouteSchemas(t *testing.T) {
	cases := []func(*Manifest){
		func(m *Manifest) { m.Schema++ },
		func(m *Manifest) { m.Routes[1].ID = m.Routes[0].ID },
		func(m *Manifest) { m.Routes[1].Name = m.Routes[0].Name },
		func(m *Manifest) { m.Routes[0].Owner = "provider" },
		func(m *Manifest) { m.Routes[0].Inputs[0].Type = "unsafe.Pointer" },
		func(m *Manifest) { m.Routes[0].Inputs[0].Type = "func()" },
		func(m *Manifest) { m.Routes[0].Inputs[0].Type = "any" },
		func(m *Manifest) { m.Routes[0].Legacy = append(m.Routes[0].Legacy, m.Routes[1].Legacy...) },
		func(m *Manifest) { m.Routes[0].Inputs[0].Name = "FD; panic()" },
	}
	for _, change := range cases {
		m, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		change(&m)
		if _, err := RenderKernel(m); err == nil {
			t.Fatal("invalid schema rendered")
		}
	}
	if _, err := Parse(append(append([]byte(nil), manifest...), []byte("{}")...)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	if _, err := Parse([]byte(`{"schema":1,"routes":[],"unknown":true}`)); err == nil {
		t.Fatal("unknown schema field accepted")
	}
}

func TestGeneratedOutputIsCurrent(t *testing.T) {
	tc, err := buildcfg.Resolve("go1.25.14")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := GenerateProfile(tc, "go125", []string{"amd64"}, out); err != nil {
		t.Fatal(err)
	}
	outputs := readOutputTree(t, out)
	for name, want := range outputs {
		got, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("stale generated output %s; run gosim-gen", name)
		}
	}
}

func TestPinnedGenerationIsScopedAndRepeatable(t *testing.T) {
	tc, err := buildcfg.Resolve("go1.25.14")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	protected := filepath.Join(out, "internal/compat/go126/profile/routes_arm64.json")
	if err := os.MkdirAll(filepath.Dir(protected), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protected, []byte("unselected-family"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := GenerateProfile(tc, "go125", []string{"amd64"}, out); err != nil {
		t.Fatal(err)
	}
	first := readOutputTree(t, out)
	file := filepath.Join(out, "internal/kernel/routes_gen.go")
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := GenerateProfile(tc, "go125", []string{"amd64"}, out); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, readOutputTree(t, out)) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("repeat generation changed bytes or unchanged file mtime")
	}
	if string(first["internal/compat/go126/profile/routes_arm64.json"]) != "unselected-family" {
		t.Fatal("modified unselected family")
	}
	if _, err := os.Stat(filepath.Join(out, "internal/compat/go125/profile/routes_arm64.json")); !os.IsNotExist(err) {
		t.Fatal("modified unselected architecture")
	}
	var record GenerationRecord
	if err := json.Unmarshal(first["internal/compat/go125/profile/routes_amd64.json"], &record); err != nil {
		t.Fatal(err)
	}
	if record.Stage != "typed-adapters" || !buildspec.ValidDigest(record.AdapterDigest) || record.Toolchain != "go1.25.14" || len(record.Modules) == 0 || !buildspec.ValidDigest(record.OriginDigest) {
		t.Fatalf("generation identity=%+v", record)
	}
	for _, test := range []struct {
		family, patch string
		arches        []string
	}{
		{"go126", "go1.25.14", []string{"amd64"}},
		{"go125", "go1.25.13", []string{"amd64"}},
		{"go125", "go1.25.14", nil},
		{"go125", "go1.25.14", []string{"386"}},
		{"go125", "go1.25.14", []string{"amd64", "amd64"}},
	} {
		bad := tc
		bad.Version = test.patch
		if err := GenerateProfile(bad, test.family, test.arches, out); err == nil {
			t.Fatalf("invalid request admitted: %+v", test)
		}
	}
	bad := tc
	bad.Executable = "/missing/go"
	if err := GenerateProfile(bad, "go125", []string{"amd64"}, out); err == nil {
		t.Fatal("unverified executable admitted")
	}
	if !reflect.DeepEqual(first, readOutputTree(t, out)) {
		t.Fatal("failed generation mutated outputs")
	}
}

func readOutputTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(name)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
