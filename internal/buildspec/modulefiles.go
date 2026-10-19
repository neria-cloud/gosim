package buildspec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// ModuleFiles flattens Go's effective module/workspace selection into a private
// module. It never edits a caller module and never consults an install-time
// framework graph. The resulting module builds with GOWORK=off.
func ModuleFiles(r Resolution, modules []Module, modulePath string, additionalRoots ...string) (map[string][]byte, error) {
	if err := ValidateVersion(r.Toolchain.Version); err != nil {
		return nil, err
	}
	mod := new(modfile.File)
	if err := mod.AddModuleStmt(modulePath); err != nil {
		return nil, err
	}
	if err := mod.AddGoStmt(strings.TrimPrefix(r.Toolchain.Version, "go")); err != nil {
		return nil, err
	}
	// A workspace main has no selected version. Give its local replacement at
	// least the highest version required by any selected dependency, otherwise
	// MVS would upgrade our placeholder and reject a read-only build.
	versions := make(map[string]string)
	required := make(map[string]bool)
	for _, m := range modules {
		if m.Main {
			required[m.Path] = true
			_, major, ok := module.SplitPathVersion(m.Path)
			if !ok {
				return nil, fmt.Errorf("invalid main module path %s", m.Path)
			}
			if major == "" {
				major = "v0"
			} else {
				major = strings.TrimPrefix(strings.TrimPrefix(major, "/"), ".")
			}
			versions[m.Path] = major + ".0.0"
		}
	}
	sums := make(map[string]bool)
	readSums := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line != "" {
				sums[line] = true
			}
		}
		return nil
	}
	for _, m := range modules {
		effective := m.Effective()
		if effective.GoMod != "" {
			data, err := os.ReadFile(effective.GoMod)
			if err != nil {
				return nil, err
			}
			selected, err := modfile.Parse(effective.GoMod, data, nil)
			if err != nil {
				return nil, err
			}
			for _, req := range selected.Require {
				if m.Main || slices.Contains(additionalRoots, m.Path) {
					required[req.Mod.Path] = true
				}
				if old, main := versions[req.Mod.Path]; main && semver.Compare(req.Mod.Version, old) > 0 {
					versions[req.Mod.Path] = req.Mod.Version
				}
			}
			if m.Main {
				for _, exclude := range selected.Exclude {
					if err := mod.AddExclude(exclude.Mod.Path, exclude.Mod.Version); err != nil {
						return nil, err
					}
				}
			}
		}
		if m.Main || (m.Replace != nil && effective.Version == "") {
			if err := readSums(filepath.Join(effective.Dir, "go.sum")); err != nil {
				return nil, err
			}
		}
		if effective.Version != "" {
			if effective.Sum != "" {
				sums[effective.Path+" "+effective.Version+" "+effective.Sum] = true
			}
			if effective.GoModSum != "" {
				sums[effective.Path+" "+effective.Version+"/go.mod "+effective.GoModSum] = true
			}
		}
	}
	if InWorkspace(r) {
		if err := readSums(r.Context.GoWork + ".sum"); err != nil {
			return nil, err
		}
	}
	for _, m := range modules {
		if m.Path == modulePath {
			return nil, fmt.Errorf("private module path collides with selected module %s", modulePath)
		}
		version := m.Version
		if m.Main {
			version = versions[m.Path]
		}
		if !semver.IsValid(version) {
			return nil, fmt.Errorf("invalid selected version %s@%s", m.Path, version)
		}
		// Promoting every transitive module to a graph root would expand Go's
		// pruned graph and can introduce dependencies the caller never selected.
		if required[m.Path] {
			if err := mod.AddRequire(m.Path, version); err != nil {
				return nil, err
			}
		}
		effective := m.Effective()
		if m.Main || m.Replace != nil {
			path, version := effective.Path, effective.Version
			if m.Main || version == "" {
				if !filepath.IsAbs(effective.Dir) {
					return nil, fmt.Errorf("local selected module %s has no absolute directory", m.Path)
				}
				path, version = effective.Dir, ""
			}
			if err := mod.AddReplace(m.Path, "", path, version); err != nil {
				return nil, err
			}
		}
	}
	mod.SortBlocks()
	data, err := mod.Format()
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(sums))
	for line := range sums {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	return map[string][]byte{"go.mod": data, "go.sum": []byte(strings.Join(lines, "\n") + "\n")}, nil
}

// MatchModuleGraph rejects dependency upgrades or replacement changes in a
// private module. Main modules become explicit local replacements; their
// synthetic version is immaterial, but their actual selected directory is not.
func MatchModuleGraph(input, output []Module, privatePath string) error {
	want := make(map[string]Module, len(input))
	for _, m := range input {
		want[m.Path] = m
	}
	for _, got := range output {
		if got.Main && got.Path == privatePath {
			continue
		}
		m, ok := want[got.Path]
		if !ok {
			return fmt.Errorf("unselected module in generated graph: %s", got.Path)
		}
		delete(want, got.Path)
		if !m.Main && m.Version != got.Version {
			return fmt.Errorf("selected module version changed for %s", m.Path)
		}
		a, b := m.Effective(), got.Effective()
		if m.Main || (m.Replace != nil && a.Version == "") {
			if got.Replace == nil || b.Version != "" || a.Dir == "" || b.Dir == "" {
				return fmt.Errorf("local module selection changed for %s", m.Path)
			}
			x, err := filepath.EvalSymlinks(a.Dir)
			if err != nil {
				return err
			}
			y, err := filepath.EvalSymlinks(b.Dir)
			if err != nil {
				return err
			}
			if x != y {
				return fmt.Errorf("local module directory changed for %s", m.Path)
			}
		} else if m.Version != got.Version || a.Path != b.Path || a.Version != b.Version {
			return fmt.Errorf("module version/replacement changed for %s", m.Path)
		}
	}
	if len(want) != 0 {
		var missing []string
		for path := range want {
			missing = append(missing, path)
		}
		slices.Sort(missing)
		return fmt.Errorf("selected modules missing from generated graph: %s", strings.Join(missing, ", "))
	}
	return nil
}
