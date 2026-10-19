package routegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// CheckProfile validates and renders into a temporary tree, then compares only
// the requested outputs. It never repairs stale files or changes their mtimes.
func CheckProfile(tc buildspec.Toolchain, profileID string, arches []string, output string) error {
	if output == "" {
		return fmt.Errorf("explicit output is required")
	}
	staging, err := os.MkdirTemp("", "gosim-gen-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := GenerateProfile(tc, profileID, arches, staging); err != nil {
		return err
	}
	return filepath.WalkDir(staging, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		expected, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		actual, err := os.ReadFile(filepath.Join(output, rel))
		if err != nil {
			return fmt.Errorf("check generated %s: %w", rel, err)
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("stale generated output %s", rel)
		}
		return nil
	})
}
