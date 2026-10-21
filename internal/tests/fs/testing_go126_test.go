//go:build go1.26

package behavior_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type artifactTesting interface{ testing.TB }

func TestTestingArtifactAndTempRoot(t *testing.T) {
	// Simulated disks start empty; provision the temporary parent explicitly.
	if err := os.MkdirAll(os.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	t.Setenv("GOTMPDIR", root)
	var artifact, temporary string
	t.Run("child", func(t *testing.T) {
		var tb artifactTesting = t
		artifact, temporary = tb.ArtifactDir(), tb.TempDir()
		if artifact != tb.ArtifactDir() || artifact == temporary {
			t.Fatal("artifact or temporary directory identity")
		}
		for _, path := range []string{artifact, temporary} {
			relative, err := filepath.Rel(root, path)
			if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				t.Fatal("GOTMPDIR ignored", path)
			}
			file := filepath.Join(path, "result")
			if err := os.WriteFile(file, []byte("artifact"), 0o600); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(file); err != nil || string(data) != "artifact" {
				t.Fatal("artifact readback", err)
			}
			// TODO(not planned): recursive RemoveAll uses directory-relative FDs,
			// which the filesystem model rejects. Exercise automatic directory
			// cleanup after removing the content through supported absolute paths.
			tb.Cleanup(func() {
				if err := os.Remove(file); err != nil {
					t.Error(err)
				}
			})
		}
		tb.Cleanup(func() {
			if tb.Context().Err() == nil {
				t.Error("cleanup preceded context cancellation")
			}
		})
	})
	for _, path := range []string{artifact, temporary} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("directory survived child cleanup", path, err)
		}
	}
	t.Run("sibling", func(t *testing.T) {
		if t.ArtifactDir() == artifact {
			t.Fatal("artifact directory reused across tests")
		}
	})
}
