//go:build go1.26

package testing

import (
	"os"
	"path/filepath"
	native "testing"
)

func TestArtifactDirReuseIsolationAndCleanup(t *native.T) {
	t.Setenv("GOTMPDIR", t.TempDir())
	var first, second T
	t.Cleanup(func() { first.runCleanup(normalPanic); second.runCleanup(normalPanic) })
	a, b := first.ArtifactDir(), second.ArtifactDir()
	if a == b || a != first.ArtifactDir() {
		t.Fatal("artifact directory identity")
	}
	if filepath.Dir(a) != os.Getenv("GOTMPDIR") {
		t.Fatal("GOTMPDIR ignored")
	}
	if err := os.WriteFile(filepath.Join(a, "output"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	first.runCleanup(normalPanic)
	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Fatal("artifact directory survived cleanup", err)
	}
	if _, err := os.Stat(b); err != nil {
		t.Fatal("other test's artifact removed", err)
	}
}
