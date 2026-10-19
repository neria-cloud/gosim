package buildcfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

const GosimModule = "github.com/neria-cloud/gosim"

func FrameworkModule(modules []buildspec.Module) (buildspec.Module, error) {
	for _, m := range modules {
		if m.Path == GosimModule {
			selected := m.Effective()
			if selected.Dir == "" {
				return buildspec.Module{}, fmt.Errorf("selected Gosim source is unavailable; download the module with Go tooling")
			}
			if !filepath.IsAbs(selected.Dir) {
				return buildspec.Module{}, fmt.Errorf("selected Gosim source must be absolute")
			}
			if _, err := os.Stat(filepath.Join(selected.Dir, "go.mod")); err != nil {
				return buildspec.Module{}, fmt.Errorf("selected Gosim source: %w", err)
			}
			return m, nil
		}
	}
	return buildspec.Module{}, fmt.Errorf("application module graph does not contain %s; add it as a Go dependency", GosimModule)
}

// GraphDigest binds selected versions and actual local module contents. It is
// used for application artifacts; workerboot separately fingerprints only its
// selected dependency packages, so editing unrelated application code does not
// force a worker rebuild.
func GraphDigest(modules []buildspec.Module) (string, error) {
	type entry struct {
		Path, Version, Sum, GoModSum, GoVersion string
		Main                                    bool
		Replacement                             string
		LocalDigest                             string
	}
	entries := make([]entry, 0, len(modules))
	for _, m := range modules {
		effective := m.Effective()
		e := entry{Path: m.Path, Version: m.Version, Sum: effective.Sum, GoModSum: effective.GoModSum, GoVersion: effective.GoVersion, Main: m.Main}
		if m.Replace != nil {
			e.Replacement = effective.Path + "@" + effective.Version
			if effective.Version == "" {
				e.Replacement = "local"
			}
		}
		if m.Main || (m.Replace != nil && effective.Version == "") {
			digest, err := HashSources(effective.Dir)
			if err != nil {
				return "", err
			}
			e.LocalDigest = digest
		}
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.Path, b.Path) })
	data, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return buildspec.Hash(data), nil
}

func HashSources(root string) (string, error) { return hashSources(root, []string{"."}) }

func HashFile(path string) (string, error) { return hashFile(path) }
