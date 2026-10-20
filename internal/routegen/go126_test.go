package routegen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestSelectedGenerationGatesAndIsolation(t *testing.T) {
	t.Parallel()
	tc, profile, options := currentGeneration(t)
	other := otherFamily(profile.ID)
	output := t.TempDir()
	protected := filepath.Join(output, "internal/compat/"+other+"/profile/routes_amd64.json")
	if err := os.MkdirAll(filepath.Dir(protected), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protected, []byte("other family"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := GenerateProfile(tc, profile.ID, []string{"amd64"}, output, options); err != nil {
		t.Fatal(err)
	}
	first := readOutputTree(t, output)
	for name, want := range first {
		if strings.Contains(name, "/"+other+"/") {
			if string(want) != "other family" {
				t.Fatal("changed other family")
			}
			continue
		}
		assertGeneratedMatchesBaseline(t, tc, name, want)
		got := want
		if strings.HasSuffix(name, ".go") && strings.Contains(name, "/"+profile.ID+"/") {
			for _, forbidden := range []string{"/internal/compat/" + other + "/", "/internal/simulation", "/internal/fs", "/internal/net", "/internal/hooks/", "/gosimruntime"} {
				if bytes.Contains(got, []byte(forbidden)) {
					t.Fatalf("%s imports %s", name, forbidden)
				}
			}
		}
	}
	file := filepath.Join(output, "internal/compat/"+profile.ID+"/hooks/fs/syscalls_amd64.go")
	stat, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := GenerateProfile(tc, profile.ID, []string{"amd64"}, output, options); err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if stat.ModTime() != second.ModTime() {
		t.Fatal("unchanged output was rewritten")
	}
	for name, want := range first {
		got, err := os.ReadFile(filepath.Join(output, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("generation changed %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "internal/compat/"+profile.ID+"/hooks/fs/syscalls_arm64.go")); !os.IsNotExist(
		err,
	) {
		t.Fatal("unselected architecture emitted")
	}
	manifest, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	profile.Hooks = append([]spec.Hook(nil), profile.Hooks...)
	for i, h := range profile.Hooks {
		if h.Package == "syscall" && h.Symbol == "Fstat" {
			profile.Hooks[i].Signature = "func(int) error"
		}
	}
	if _, err := renderAdapters(tc, profile, "amd64", manifest); err == nil ||
		!strings.Contains(err.Error(), "unreviewed adapter ABI") {
		t.Fatalf("signature mutation: %v", err)
	}
	// Optional-domain gates precede all argument marshalling, even invalid pointers.
	for _, owner := range []string{"fs", "net"} {
		path := filepath.Join(output, "internal/compat/"+profile.ID+"/hooks", owner, "syscalls_amd64.go")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range profile.Hooks {
			if h.Owner != owner || (h.Package != "syscall" && h.Package != "golang.org/x/sys/unix") {
				continue
			}
			name := adapterPrefix(h.Package) + h.Symbol
			start := strings.Index(string(data), "func "+name+"(")
			if start < 0 {
				t.Fatalf("missing wrapper %s", name)
			}
			body := string(data)[start:]
			brace := strings.Index(body, "{\n")
			if brace < 0 || !strings.HasPrefix(strings.TrimSpace(body[brace+2:]), "kh.Require(") {
				t.Fatalf("missing first gate: %s", name)
			}
		}
	}
}

// Listener capability probes are handwritten adapters, not generated syscall
// wrappers. Check their first-operation gate alongside the generated wrappers.
func TestListenerProbeGate(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"go125", "go126"} {
		data, err := os.ReadFile(filepath.Join("../../internal/compat", family, "hooks/net/listen.go"))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		at := strings.Index(source, "func Net_maxListenerBacklog() int {")
		if at < 0 {
			t.Fatal("missing listener probe", family)
		}
		body := source[at:]
		brace := strings.Index(body, "{\n")
		if brace < 0 || !strings.HasPrefix(strings.TrimSpace(body[brace+2:]), "kh.Require(contracts.Net,") {
			t.Fatal("listener probe lacks first-operation gate", family)
		}
		if !strings.Contains(body, "return contracts.ListenerBacklog") {
			t.Fatal("listener probe detached from model capacity", family)
		}
	}
}
