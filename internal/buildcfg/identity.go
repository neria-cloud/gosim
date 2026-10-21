package buildcfg

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func Canonical(id buildspec.Identity) ([]byte, error) { return buildspec.Canonical(id) }
func Digest(id buildspec.Identity) (string, error)    { return buildspec.Digest(id) }

// Identity binds already-fingerprinted compiler inputs and checked contracts.
// Fingerprint inventories are passed through once per operation.
func Identity(ctx context.Context, cfg buildspec.Config, resolution buildspec.Resolution, profile spec.Profile, opts buildspec.BuildOptions, inputs buildspec.Fingerprints, contractDigest string) (buildspec.Identity, error) {
	tc := resolution.Toolchain
	cfg, err := Normalize(cfg)
	if err != nil {
		return buildspec.Identity{}, err
	}
	if err := buildspec.CheckEnvironment(tc); err != nil {
		return buildspec.Identity{}, err
	}
	if err := profile.Validate(); err != nil {
		return buildspec.Identity{}, err
	}
	if opts.Experiments != "" {
		return buildspec.Identity{}, fmt.Errorf("nondefault experiments are unsupported")
	}
	for _, tag := range opts.Tags {
		if tag != "sim" && tag != "linkname" {
			return buildspec.Identity{}, fmt.Errorf("unapproved build tag %q", tag)
		}
	}
	tags := slices.Clone(opts.Tags)
	slices.Sort(tags)
	if len(slices.Compact(tags)) != len(opts.Tags) {
		return buildspec.Identity{}, fmt.Errorf("duplicate build tags")
	}
	expected, err := profile.ContractDigest()
	if err != nil {
		return buildspec.Identity{}, err
	}
	if contractDigest != expected || !buildspec.ValidDigest(inputs.ToolchainDigest) || !buildspec.ValidDigest(inputs.StdlibDigest) {
		return buildspec.Identity{}, fmt.Errorf("missing or mismatched checked compatibility inputs")
	}
	modules, err := buildspec.ResolveModuleGraph(ctx, resolution)
	if err != nil {
		return buildspec.Identity{}, err
	}
	framework, err := FrameworkModule(modules)
	if err != nil {
		return buildspec.Identity{}, err
	}
	graph, err := GraphDigest(modules)
	if err != nil {
		return buildspec.Identity{}, err
	}
	return identityForSource(cfg, tc, profile, opts, inputs, framework.Effective().Dir, graph)
}

func identityForSource(cfg buildspec.Config, tc buildspec.Toolchain, profile spec.Profile, opts buildspec.BuildOptions, dist buildspec.Fingerprints, root, graph string) (buildspec.Identity, error) {
	contract, err := profile.ContractDigest()
	if err != nil {
		return buildspec.Identity{}, err
	}
	id := buildspec.Identity{
		Schema: 2, ContractDigest: contract, Toolchain: tc.Version, Profile: profile.ID, AdapterRevision: profile.Revision,
		ToolchainDigest: dist.ToolchainDigest, StdlibDigest: dist.StdlibDigest,
		HostOS: tc.HostOS, HostArch: tc.HostArch, SimulationOS: cfg.SimulationOS, SimulationArch: tc.HostArch,
		Features: cfg.Features, Race: opts.Race, Experiments: opts.Experiments,
		Tags: []string{"linkname", "sim"}, ModuleGraphDigest: graph,
	}
	id.RuntimeRevision, err = hashSources(root, []string{"rt/runtime", "internal/contracts", "internal/kernel", "internal/control"})
	if err != nil {
		return id, err
	}
	for _, feature := range cfg.Features {
		if feature == "rt" {
			continue
		}
		digest, err := hashSources(root, []string{"internal/" + feature})
		if err != nil {
			return id, err
		}
		id.ProviderRevisions = append(id.ProviderRevisions, buildspec.Revision{Name: feature, Digest: digest})
	}
	// The complete tool/source tree is a conservative translator fingerprint.
	// Evidence metadata and documentation never enter executable identities.
	id.TranslatorDigest, err = hashSources(root, []string{"."})
	if err != nil {
		return id, err
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return id, err
	}
	policy, err := json.Marshal([]string{"gosim-build-policy-v1", "-ldflags=-checklinkname=0", "-tags=linkname,sim", "GODEBUG=", string(mod)})
	if err != nil {
		return id, err
	}
	id.DebugPolicyDigest = buildspec.Hash(policy)
	return buildspec.NormalizeIdentity(id)
}

func evidenceMetadata(name string) bool { return name == "internal/compat/profiles/evidence.json" }

func hashSources(root string, paths []string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("missing source root")
	}
	var files []buildspec.Revision
	for _, sub := range paths {
		before := len(files)
		err := filepath.WalkDir(filepath.Join(root, sub), func(filename string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, filename)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel == ".git" {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				if entry.Name() == ".git" || entry.Name() == ".gosim" || rel == "docs" || rel == "test-reports" {
					return filepath.SkipDir
				}
				return nil
			}
			if evidenceMetadata(rel) {
				return nil
			}
			// Hash all non-documentation inputs, including embeds and templates.
			if filepath.Ext(rel) == ".md" {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("source input must be a regular file: %s", rel)
			}
			digest, err := hashFile(filename)
			if err != nil {
				return err
			}
			files = append(files, buildspec.Revision{Name: rel, Digest: digest})
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", sub, err)
		}
		if len(files) == before {
			return "", fmt.Errorf("no source inputs in %s", sub)
		}
	}
	slices.SortFunc(files, func(a, b buildspec.Revision) int { return strings.Compare(a.Name, b.Name) })
	data, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	return buildspec.Hash(data), nil
}
