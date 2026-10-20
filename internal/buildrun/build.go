// Package buildrun compiles an already validated immutable selection.
package buildrun

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/translate"
)

type BuildResult struct {
	Config       gosimtool.BuildConfig
	Artifacts    []gosimtool.Artifact
	Dependencies map[string]map[string]string
}

func BuildResolved(cfg gosimtool.BuildConfig, patterns []string) (result BuildResult, err error) {
	if err := cfg.Validate(); err != nil {
		return result, err
	}
	result.Config = cfg
	output, err := translate.Translate(&translate.TranslateInput{Packages: patterns, Cfg: result.Config})
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(output.RootOutputDir)) }()
	result.Dependencies = output.Deps
	project, err := cfg.ProjectRoot()
	if err != nil {
		return result, err
	}
	files, err := readFiles(output.RootOutputDir)
	if err != nil {
		return result, err
	}
	for _, pkg := range output.Packages {
		input, err := gosimtool.PackageSourceDigest(pkg, output.Deps[pkg])
		if err != nil {
			return result, err
		}
		inputs := []gosimtool.PackageDigest{input}
		artifact, err := gosimtool.BuildArtifact(cfg.Context, project, result.Config.Identity, inputs, func(stage string) (gosimtool.Manifest, error) {
			manifest := gosimtool.Manifest{Schema: 2, Identity: result.Config.Identity, PackageDigests: inputs}
			source := filepath.Join(stage, "source")
			if err := os.Mkdir(source, 0o755); err != nil {
				return manifest, err
			}
			if err := gosimtool.MaterializeFiles(source, files); err != nil {
				return manifest, err
			}
			if err := validateOutputGraph(result.Config, source, pkg); err != nil {
				return manifest, err
			}
			args := []string{"test", "-c", "-trimpath", "-mod=readonly", "-ldflags=-checklinkname=0", "-tags=linkname,sim", "-o", filepath.Join(stage, "testbin")}
			if cfg.Identity.Race {
				args = append(args, "-race")
			}
			args = append(args, pkg)
			cmd := buildspec.OutputCommand(cfg.Context, result.Config.Toolchain, source, args...)
			cmd.Env = replaceEnv(cmd.Env, "CGO_ENABLED", "1")
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := buildspec.RunProcess(cfg.Context, cmd); err != nil {
				return manifest, fmt.Errorf("compile candidate %s: %w\n%s", pkg, err, output.Bytes())
			}
			built, err := readFiles(stage)
			if err != nil {
				return manifest, err
			}
			for name, data := range built {
				file := gosimtool.ArtifactFile{Path: name, SHA256: buildspec.Hash(data)}
				if name == "testbin" {
					manifest.Binary = file
				} else {
					manifest.GeneratedFiles = append(manifest.GeneratedFiles, file)
				}
			}
			return manifest, nil
		})
		if err != nil {
			return result, err
		}
		result.Artifacts = append(result.Artifacts, artifact)
	}
	return result, nil
}

// Check the compiler's actual dependency graph, including native islands and
// generated test mains, before publication. Source policies already check the
// original Linux graph; this prevents a rewrite from adding a forbidden model.
func validateOutputGraph(cfg gosimtool.BuildConfig, dir, pkg string) error {
	input, err := cfg.ModuleGraph()
	if err != nil {
		return err
	}
	flat := cfg.Resolution
	flat.Context = buildspec.ModuleContext{Directory: dir, GoWork: "off", GoMod: filepath.Join(dir, "go.mod")}
	output, err := buildspec.ResolveModuleGraph(cfg.Context, flat)
	if err != nil {
		return err
	}
	if err := buildspec.MatchModuleGraph(input, output, "translated"); err != nil {
		return err
	}

	args := []string{"list", "-mod=readonly", "-tags=linkname,sim", "-deps", "-test", "-json"}
	if cfg.Identity.Race {
		args = append(args, "-race")
	}
	args = append(args, pkg)
	cmd := buildspec.OutputCommand(cfg.Context, cfg.Toolchain, dir, args...)
	cmd.Env = replaceEnv(cmd.Env, "CGO_ENABLED", "1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect candidate graph: %w\n%s", err, out)
	}
	found := make(map[string]bool)
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg struct {
			ImportPath string
			Imports    []string
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		path := strings.Split(pkg.ImportPath, " ")[0]
		translatedRoot := "translated/" + gosimtool.ReplaceSpecialPackages(gosimtool.Module)
		for _, prefix := range []string{gosimtool.Module + "/internal/compat/", translatedRoot + "/internal_/compat/"} {
			if suffix, ok := strings.CutPrefix(path, prefix); ok {
				family, _, _ := strings.Cut(suffix, "/")
				if strings.HasPrefix(family, "go") && family != cfg.Identity.Profile {
					return fmt.Errorf("inactive family in compiled graph: %s", path)
				}
			}
		}
		for _, imported := range pkg.Imports {
			for _, provider := range []string{"fs", "net"} {
				target := translatedRoot + "/internal_/" + provider
				if imported != target && !strings.HasPrefix(imported, target+"/") {
					continue
				}
				// Domain models may import their own provider subtree; composition owns
				// construction. No other production package gains a concrete dependency.
				forbidden := path == translatedRoot || path == translatedRoot+"/fs" || path == translatedRoot+"/net" || path == translatedRoot+"/rt"
				for _, consumer := range []string{"contracts", "control", "kernel"} {
					prefix := translatedRoot + "/internal_/" + consumer
					forbidden = forbidden || path == prefix || strings.HasPrefix(path, prefix+"/")
				}
				peer := "fs"
				if provider == "fs" {
					peer = "net"
				}
				prefix := translatedRoot + "/internal_/" + peer
				forbidden = forbidden || path == prefix || strings.HasPrefix(path, prefix+"/")
				if forbidden {
					return fmt.Errorf("forbidden compiled provider dependency: %s -> %s", path, imported)
				}
			}
		}
		for _, feature := range []string{"fs", "net"} {
			original := gosimtool.Module + "/internal/" + feature
			translated := "translated/" + gosimtool.ReplaceSpecialPackages(original)
			if path == original || strings.HasPrefix(path, original+"/") {
				return fmt.Errorf("native provider escaped translation: %s", path)
			}
			if path == translated || strings.HasPrefix(path, translated+"/") {
				if !slices.Contains(cfg.Identity.Features, feature) {
					return fmt.Errorf("disabled provider in compiled graph: %s", path)
				}
				found[feature] = true
			}
		}
	}
	for _, feature := range cfg.Identity.Features {
		if feature != "rt" && !found[feature] {
			return fmt.Errorf("selected provider %s missing from compiled graph", feature)
		}
	}
	return nil
}

func replaceEnv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, key+"=") {
			out = append(out, entry)
		}
	}
	return append(out, key+"="+value)
}

func readFiles(root string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular generated file %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = slices.Clone(data)
		return nil
	})
	return files, err
}
