package buildcfg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

const GosimModule = "github.com/neria-cloud/gosim"

// Module records the graph Go selected, including effective local replacements
// and every workspace main module. Locations are not executable identity.
type Module struct {
	Path      string
	Version   string
	Sum       string
	GoModSum  string
	GoVersion string
	Main      bool
	Dir       string
	GoMod     string
	Replace   *Module
}

func (m Module) Effective() Module {
	if m.Replace != nil {
		return *m.Replace
	}
	return m
}

func InWorkspace(r buildspec.Resolution) bool {
	return r.Context.GoWork != "" && r.Context.GoWork != "off"
}

func ApplicationFlags(r buildspec.Resolution) []string {
	if InWorkspace(r) {
		return nil
	}
	return []string{"-mod=readonly"}
}

func ResolveModuleGraph(ctx context.Context, r buildspec.Resolution) ([]Module, error) {
	args := append([]string{"list"}, ApplicationFlags(r)...)
	args = append(args, "-m", "-json", "all")
	cmd := buildspec.ApplicationCommand(ctx, r, args...)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("resolve application module graph: %w\n%s", err, diagnostics.Bytes())
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var modules []Module
	seen := map[string]bool{}
	for {
		var m Module
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if m.Path == "" || seen[m.Path] {
			return nil, fmt.Errorf("invalid/duplicate selected module %q", m.Path)
		}
		seen[m.Path] = true
		modules = append(modules, m)
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("no application module graph; run inside a Go module or workspace")
	}
	slices.SortFunc(modules, func(a, b Module) int { return strings.Compare(a.Path, b.Path) })
	return modules, nil
}

func FrameworkModule(modules []Module) (Module, error) {
	for _, m := range modules {
		if m.Path == GosimModule {
			selected := m.Effective()
			if selected.Dir == "" {
				return Module{}, fmt.Errorf("selected Gosim source is unavailable; download the module with Go tooling")
			}
			if !filepath.IsAbs(selected.Dir) {
				return Module{}, fmt.Errorf("selected Gosim source must be absolute")
			}
			if _, err := os.Stat(filepath.Join(selected.Dir, "go.mod")); err != nil {
				return Module{}, fmt.Errorf("selected Gosim source: %w", err)
			}
			return m, nil
		}
	}
	return Module{}, fmt.Errorf("application module graph does not contain %s; add it as a Go dependency", GosimModule)
}

// GraphDigest binds selected versions and actual local module contents. It is
// used for application artifacts; workerboot separately fingerprints only its
// selected dependency packages, so editing unrelated application code does not
// force a worker rebuild.
func GraphDigest(modules []Module) (string, error) {
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
