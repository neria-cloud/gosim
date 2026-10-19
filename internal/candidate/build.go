// Package candidate is the internal pending-tuple build/run path. It cannot
// promote compatibility evidence or produce a certified artifact.
package candidate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/translate"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

type BuildResult struct {
	Config       gosimtool.BuildConfig
	Artifacts    []gosimtool.Artifact
	Dependencies map[string]map[string]string
}

func Resolve(config buildspec.Config, options buildspec.BuildOptions) (gosimtool.BuildConfig, error) {
	config, err := buildcfg.Normalize(config)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	tc, err := buildcfg.Resolve(config.Toolchain)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	if err := buildcfg.ValidateWorker(tc); err != nil {
		return gosimtool.BuildConfig{}, err
	}
	profile := selected.Profile()
	id, err := buildcfg.Identity(config, tc, profile, options)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	if id.Certification != "candidate" {
		return gosimtool.BuildConfig{}, fmt.Errorf("candidate resolver requires unpromoted identity")
	}
	selection, err := profile.Select(id)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	if err := spec.VerifyHooks(tc, profile.Hooks); err != nil {
		return gosimtool.BuildConfig{}, err
	}
	return gosimtool.NewBuildConfig(selection, tc)
}

func Build(config buildspec.Config, options buildspec.BuildOptions, patterns []string) (result BuildResult, err error) {
	result.Config, err = Resolve(config, options)
	if err != nil {
		return result, err
	}
	return BuildResolved(result.Config, patterns)
}

// BuildResolved uses the same validated identity for every command in a batch.
func BuildResolved(cfg gosimtool.BuildConfig, patterns []string) (result BuildResult, err error) {
	if err := cfg.Validate(); err != nil {
		return result, err
	}
	if cfg.Identity.Certification != "candidate" {
		return result, fmt.Errorf("candidate builder requires unpromoted identity")
	}
	result.Config = cfg
	output, err := translate.Translate(&translate.TranslateInput{Packages: patterns, Cfg: result.Config})
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(output.RootOutputDir)) }()
	result.Dependencies = output.Deps
	project, err := gosimtool.FindGoModDir()
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
		artifact, err := gosimtool.BuildArtifact(project, result.Config.Identity, inputs, func(stage string) (gosimtool.Manifest, error) {
			manifest := gosimtool.Manifest{Schema: 1, Identity: result.Config.Identity, PackageDigests: inputs}
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
			cmd := buildcfg.Command(result.Config.Toolchain, source, args...)
			cmd.Env = replaceEnv(cmd.Env, "CGO_ENABLED", "1")
			if out, err := cmd.CombinedOutput(); err != nil {
				return manifest, fmt.Errorf("compile candidate %s: %w\n%s", pkg, err, out)
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

// Run verifies stored bytes before launch and returned identity before accepting
// results.
func Run(ctx context.Context, artifact gosimtool.Artifact, config simrt.RunConfig) (simrt.RunResult, error) {
	results, err := runRequests(ctx, artifact, []simrt.RunConfig{config})
	if err != nil {
		return simrt.RunResult{}, err
	}
	return results[0], nil
}

func runRequests(ctx context.Context, artifact gosimtool.Artifact, configs []simrt.RunConfig) ([]simrt.RunResult, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("candidate run requires requests")
	}
	if artifact.Manifest.Identity.Certification != "candidate" {
		return nil, fmt.Errorf("candidate runner requires a candidate artifact")
	}
	manifest, err := gosimtool.VerifyArtifact(artifact.Directory, artifact.Manifest.Identity, artifact.Manifest.PackageDigests)
	if err != nil {
		return nil, err
	}
	var requests bytes.Buffer
	digest, err := buildspec.Digest(manifest.Identity)
	if err != nil {
		return nil, err
	}
	hello := simrt.MetatestHandshake{Protocol: 2, IdentityDigest: digest, Toolchain: manifest.Identity.Toolchain}
	if err := json.NewEncoder(&requests).Encode(hello); err != nil {
		return nil, err
	}
	for _, config := range configs {
		if err := json.NewEncoder(&requests).Encode(config); err != nil {
			return nil, err
		}
	}
	cmd := exec.CommandContext(ctx, filepath.Join(artifact.Directory, manifest.Binary.Path), "-metatest")
	cmd.Stdin = &requests
	cmd.Env = replaceEnv(os.Environ(), "GODEBUG", "")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("candidate process: %w\n%s", err, stderr.Bytes())
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var peer simrt.MetatestHandshake
	if err := decoder.Decode(&peer); err != nil || peer != hello {
		return nil, fmt.Errorf("candidate protocol/identity mismatch: %v", err)
	}
	results := make([]simrt.RunResult, len(configs))
	for i := range configs {
		if err := decoder.Decode(&results[i]); err != nil {
			return nil, fmt.Errorf("decode candidate result: %w\nstdout: %s\nstderr: %s", err, data, stderr.Bytes())
		}
		if results[i].BuildDigest != digest || results[i].ReplayRevision != simrt.ReplayRevision {
			return nil, fmt.Errorf("candidate result identity mismatch")
		}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing candidate response: %v", err)
	}
	return results, nil
}

// Check the compiler's actual dependency graph, including native islands and
// generated test mains, before publication. Source policies already check the
// original Linux graph; this prevents a rewrite from adding a forbidden model.
func validateOutputGraph(cfg gosimtool.BuildConfig, dir, pkg string) error {
	args := []string{"list", "-mod=readonly", "-tags=linkname,sim", "-deps", "-test", "-f", "{{.ImportPath}}"}
	if cfg.Identity.Race {
		args = append(args, "-race")
	}
	args = append(args, pkg)
	cmd := buildcfg.Command(cfg.Toolchain, dir, args...)
	cmd.Env = replaceEnv(cmd.Env, "CGO_ENABLED", "1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect candidate graph: %w\n%s", err, out)
	}
	found := make(map[string]bool)
	for _, path := range strings.Fields(string(out)) {
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
