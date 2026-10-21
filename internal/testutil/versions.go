// Package testutil contains shared test fixtures, never runtime admission rules.
package testutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// UnregisteredGo follows the registry so onboarding does not invalidate negative tests.
func UnregisteredGo() (v struct {
	Version, Family, Profile string
	Minor                    int
},
) {
	for _, f := range buildspec.SupportedFamilies() {
		parsed, err := buildspec.ParseGoVersion("go" + f.Family + ".0")
		if err != nil {
			panic(err)
		}
		v.Minor = max(v.Minor, parsed.Minor+1)
	}
	v.Family = fmt.Sprintf("1.%d", v.Minor)
	v.Profile = fmt.Sprintf("go1%d", v.Minor)
	v.Version = "go" + v.Family + ".1"
	return v
}

// CopySource creates a disposable source checkout without artifacts or git state.
func CopySource(t testing.TB, source string) string {
	t.Helper()
	source, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	err = filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if slices.Contains([]string{".git", ".gosim", "docs", "test-reports", "__pycache__"}, entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}
