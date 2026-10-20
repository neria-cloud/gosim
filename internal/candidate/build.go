// Package candidate exposes internal build/run helpers using the same family
// support policy and artifact checks as ordinary commands.
package candidate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildrun"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

type BuildResult = buildrun.BuildResult

func Resolve(config buildspec.Config, options buildspec.BuildOptions) (gosimtool.BuildConfig, error) {
	config, err := buildcfg.Normalize(config)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	resolution, err := buildcfg.ResolveRunningProcess(context.Background(), "")
	tc := resolution.Toolchain
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	if err := buildcfg.ValidateWorker(tc); err != nil {
		return gosimtool.BuildConfig{}, err
	}
	profile := selected.Profile()
	inputs, err := buildcfg.Fingerprint(tc)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	contract, err := spec.CheckContracts(context.Background(), resolution, profile, options, inputs, "")
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	id, err := buildcfg.Identity(context.Background(), config, resolution, profile, options, inputs, contract)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	selection, err := profile.Select(id)
	if err != nil {
		return gosimtool.BuildConfig{}, err
	}
	return gosimtool.NewBuildConfig(context.Background(), selection, resolution)
}

func Build(config buildspec.Config, options buildspec.BuildOptions, patterns []string) (result BuildResult, err error) {
	result.Config, err = Resolve(config, options)
	if err != nil {
		return result, err
	}
	return BuildResolved(result.Config, patterns)
}

// BuildResolved uses the same validated identity for every command in a batch.
func BuildResolved(cfg gosimtool.BuildConfig, patterns []string) (BuildResult, error) {
	return buildrun.BuildResolved(cfg, patterns)
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

func runRequests(
	ctx context.Context,
	artifact gosimtool.Artifact,
	configs []simrt.RunConfig,
) ([]simrt.RunResult, error) {
	if len(configs) == 0 {
		return nil, errors.New("candidate run requires requests")
	}
	manifest, err := gosimtool.VerifyArtifact(
		artifact.Directory,
		artifact.Manifest.Identity,
		artifact.Manifest.PackageDigests,
	)
	if err != nil {
		return nil, err
	}
	var requests bytes.Buffer
	digest, err := buildspec.Digest(manifest.Identity)
	if err != nil {
		return nil, err
	}
	hello := simrt.MetatestHandshake{Protocol: 3, IdentityDigest: digest, Toolchain: manifest.Identity.Toolchain}
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
			return nil, errors.New("candidate result identity mismatch")
		}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing candidate response: %v", err)
	}
	return results, nil
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
