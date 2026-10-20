//go:build sim

package filesystem_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/neria-cloud/gosim"
)

func TestReadSimulatedFile(t *testing.T) {
	if version := runtime.Version(); version != "go1.25.2" {
		t.Fatalf("this example requires go1.25.2; running %s", version)
	}
	if !gosim.IsSim() {
		t.Fatal("this example must run inside Gosim")
	}

	const path = "/tmp/testfile.txt"
	const want = "Hello World!"
	// These ordinary os calls operate on Gosim's simulated filesystem.
	if err := os.MkdirAll("/tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("read %q; want %q", got, want)
	}
	t.Logf("%s: read %q using %s", path, got, runtime.Version())
}
