package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

const frameworkModule = "github.com/neria-cloud/gosim"

// inspectorDigest follows imports in every non-test source variant, not only the
// files selected by today's compiler. Generated support and all data stay out.
func inspectorDigest(ctx context.Context, root string) (string, error) {
	queue := []string{"cmd/gosim-gen", "internal/compat/audit"}
	seen := map[string]bool{}
	var inputs []buildspec.Revision
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		if seen[dir] {
			continue
		}
		seen[dir] = true
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			name := entry.Name()
			rel := filepath.ToSlash(filepath.Join(dir, name))
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || inspectorGenerated(rel) {
				continue
			}
			data, err := readRegular(filepath.Join(root, rel))
			if err != nil {
				return "", err
			}
			inputs = append(inputs, buildspec.Revision{Name: rel, Digest: buildspec.Hash(data)})
			file, err := parser.ParseFile(token.NewFileSet(), rel, data, parser.ImportsOnly)
			if err != nil {
				return "", err
			}
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return "", err
				}
				if path == frameworkModule {
					queue = append(queue, ".")
				} else if suffix, ok := strings.CutPrefix(path, frameworkModule+"/"); ok {
					queue = append(queue, suffix)
				}
			}
		}
	}
	return digestRevisions("gosim-inspector-v1", inputs), nil
}

func inspectorGenerated(path string) bool {
	if path == "internal/buildspec/families_gen.go" || path == "internal/compat/profiles/revisions_gen.go" {
		return true
	}
	if path == "internal/compat/selected/unsupported.go" || path == "internal/compat/host/unsupported.go" || path == "internal/hostabi/unsupported.go" {
		return true
	}
	minor := strings.TrimSuffix(strings.TrimPrefix(path, "internal/compat/selected/go1"), ".go")
	n, err := strconv.Atoi(minor)
	return strings.HasPrefix(path, "internal/compat/selected/go1") && strings.HasSuffix(path, ".go") && err == nil && n >= 25 && strconv.Itoa(n) == minor
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("nonregular inspection input %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	after, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, after) || info.Size() != int64(len(data)) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return nil, fmt.Errorf("inspection input changed while reading %s", path)
	}
	return data, nil
}

func digestRevisions(domain string, inputs []buildspec.Revision) string {
	slices.SortFunc(inputs, func(a, b buildspec.Revision) int { return strings.Compare(a.Name, b.Name) })
	data, err := json.Marshal(struct {
		Domain string
		Inputs []buildspec.Revision
	}{domain, inputs})
	if err != nil {
		panic(err)
	}
	return buildspec.Hash(data)
}

func moduleSourceDigest(ctx context.Context, root string) (string, error) {
	var inputs []buildspec.Revision
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := readRegular(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		inputs = append(inputs, buildspec.Revision{Name: filepath.ToSlash(rel), Digest: buildspec.Hash(data)})
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(inputs) == 0 {
		return "", fmt.Errorf("empty inspection module source %s", root)
	}
	return digestRevisions("gosim-inspection-module-v1", inputs), nil
}

// moduleInputs persists only compiler-independent caller and external inputs.
// The private generated module's exact bytes are checked separately within a scan.
func moduleInputs(ctx context.Context, r buildspec.Resolution, modules []buildspec.Module, used map[string]string) ([]buildspec.Revision, error) {
	var result []buildspec.Revision
	addFile := func(name, path string, optional bool) error {
		data, err := readRegular(path)
		if optional && os.IsNotExist(err) {
			result = append(result, buildspec.Revision{Name: name, Digest: buildspec.Hash([]byte("absent"))})
			return nil
		}
		if err != nil {
			return err
		}
		result = append(result, buildspec.Revision{Name: name, Digest: buildspec.Hash(data)})
		return nil
	}
	type graphRow struct {
		Path, Version, Replacement, ReplacementVersion string
		Main                                           bool
	}
	var graph []graphRow
	known := map[string]bool{}
	for _, m := range modules {
		if known[m.Path] {
			return nil, fmt.Errorf("duplicate inspection module %s", m.Path)
		}
		known[m.Path] = true
		row := graphRow{Path: m.Path, Version: m.Version, Main: m.Main}
		if m.Replace != nil {
			row.Replacement, row.ReplacementVersion = m.Replace.Path, m.Replace.Version
			if row.ReplacementVersion == "" {
				row.Replacement = "local"
			}
		}
		graph = append(graph, row)
		if m.Main || m.Path == frameworkModule {
			if err := addFile("module/"+m.Path+"/go.mod", m.Effective().GoMod, false); err != nil {
				return nil, err
			}
			if err := addFile("module/"+m.Path+"/go.sum", filepath.Join(m.Effective().Dir, "go.sum"), true); err != nil {
				return nil, err
			}
		}
		if dir, ok := used[m.Path]; ok && m.Path != frameworkModule {
			a, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return nil, err
			}
			b, err := filepath.EvalSymlinks(m.Effective().Dir)
			if err != nil {
				return nil, err
			}
			if a != b {
				return nil, fmt.Errorf("inspection module directory mismatch %s", m.Path)
			}
			digest, err := moduleSourceDigest(ctx, dir)
			if err != nil {
				return nil, err
			}
			result = append(result, buildspec.Revision{Name: "source/" + m.Path, Digest: digest})
		}
	}
	for path := range used {
		if !known[path] {
			return nil, fmt.Errorf("unselected inspection module %s", path)
		}
	}
	if buildspec.InWorkspace(r) {
		if err := addFile("caller/go.work", r.Context.GoWork, false); err != nil {
			return nil, err
		}
		if err := addFile("caller/go.work.sum", r.Context.GoWork+".sum", true); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(graph, func(a, b graphRow) int { return strings.Compare(a.Path, b.Path) })
	data, err := json.Marshal(graph)
	if err != nil {
		return nil, err
	}
	result = append(result, buildspec.Revision{Name: "selected-module-graph", Digest: buildspec.Hash(data)})
	slices.SortFunc(result, func(a, b buildspec.Revision) int { return strings.Compare(a.Name, b.Name) })
	return result, ctx.Err()
}
