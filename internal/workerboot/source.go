// Package workerboot resolves and starts native Go workers. It must not import
// executable compatibility adapters, the simulation engine, or the translator.
package workerboot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

const WorkerPackage = buildcfg.GosimModule + "/cmd/gosim-worker"

type Source struct {
	Directory     string `json:"directory"`
	ModuleVersion string `json:"module_version"`
	SourceDigest  string `json:"source_digest"`
	GraphDigest   string `json:"graph_digest"`
}

func ResolveSource(ctx context.Context, r buildspec.Resolution) (Source, error) {
	modules, err := buildcfg.ResolveModuleGraph(ctx, r)
	if err != nil {
		return Source{}, err
	}
	mod, err := buildcfg.FrameworkModule(modules)
	if err != nil {
		return Source{}, err
	}
	source := Source{Directory: mod.Effective().Dir, ModuleVersion: mod.Version}
	source.SourceDigest, err = buildcfg.HashSources(source.Directory)
	if err != nil {
		return Source{}, err
	}
	source.GraphDigest, err = workerGraphDigest(ctx, r)
	if err != nil {
		return Source{}, err
	}
	return source, nil
}

func workerGraphDigest(ctx context.Context, r buildspec.Resolution) (string, error) {
	args := append([]string{"list"}, buildcfg.ApplicationFlags(r)...)
	args = append(args, "-deps", "-json", "-tags=linkname", WorkerPackage)
	cmd := buildspec.ApplicationCommand(ctx, r, args...)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("load selected Gosim worker dependencies (set up dependencies with Go tooling): %w\n%s", err, diagnostics.Bytes())
	}
	type entry struct {
		ImportPath  string
		Module      string
		Version     string
		Replacement string
		Files       []buildspec.Revision
	}
	var entries []entry
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var p struct {
			ImportPath, Dir                                                                                                     string
			Standard                                                                                                            bool
			GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles, EmbedFiles []string
			Module                                                                                                              *buildcfg.Module
		}
		if err := decoder.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}
		if p.Standard {
			continue
		} // Bound by the installed toolchain/stdlib fingerprints.
		if p.Module == nil {
			return "", fmt.Errorf("worker dependency %s has no selected module", p.ImportPath)
		}
		e := entry{ImportPath: p.ImportPath, Module: p.Module.Path, Version: p.Module.Version}
		effective := p.Module.Effective()
		if p.Module.Replace != nil {
			e.Replacement = effective.Path + "@" + effective.Version
			if effective.Version == "" {
				e.Replacement = "local"
			}
		}
		names := []string{}
		for _, list := range [][]string{p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.HFiles, p.FFiles, p.SFiles, p.SwigFiles, p.SwigCXXFiles, p.SysoFiles, p.EmbedFiles} {
			names = append(names, list...)
		}
		slices.Sort(names)
		names = slices.Compact(names)
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			path := filepath.Join(p.Dir, name)
			digest, err := buildcfg.HashFile(path)
			if err != nil {
				return "", err
			}
			rel, err := filepath.Rel(effective.Dir, path)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", fmt.Errorf("worker dependency escaped selected module: %s", path)
			}
			e.Files = append(e.Files, buildspec.Revision{Name: filepath.ToSlash(rel), Digest: digest})
		}
		// Module declarations affect imports even when no source file changed.
		// Selected dependency content is already bound above; go.sum itself is
		// download evidence, not an additional executable input.
		if effective.GoMod != "" {
			digest, err := buildcfg.HashFile(effective.GoMod)
			if err != nil {
				return "", err
			}
			e.Files = append(e.Files, buildspec.Revision{Name: "go.mod", Digest: digest})
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("empty selected worker dependency graph")
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.ImportPath, b.ImportPath) })
	encoded, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return buildspec.Hash(encoded), nil
}
