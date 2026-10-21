//go:build go1.25 && !go1.26

package behavior_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTestingTempDirBaseline(t *testing.T) {
	// Simulated disks start empty; provision the temporary parent explicitly.
	if err := os.MkdirAll(os.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	t.Setenv("GOTMPDIR", root)
	var temporary string
	t.Run("child", func(t *testing.T) {
		temporary = t.TempDir()
		if filepath.Dir(temporary) == root {
			t.Fatal("Go 1.25 TempDir unexpectedly follows GOTMPDIR")
		}
		if _, err := os.Stat(temporary); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatal("temporary directory survived child cleanup", err)
	}
}
