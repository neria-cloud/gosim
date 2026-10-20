package spec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type compatibilityInputs struct {
	Schema          int                    `json:"schema"`
	Family          string                 `json:"family"`
	ContractDigest  string                 `json:"contract_digest"`
	CheckerRevision string                 `json:"checker_revision"`
	Toolchain       string                 `json:"toolchain"`
	Fingerprints    buildspec.Fingerprints `json:"fingerprints"`
	HostOS          string                 `json:"host_os"`
	Architecture    string                 `json:"architecture"`
	Race            bool                   `json:"race"`
	Experiments     string                 `json:"experiments"`
	SourceTags      string                 `json:"source_tags"`
	Modules         []buildspec.Revision   `json:"modules"`
}

type compatibilityRecord struct {
	Schema                 int                 `json:"schema"`
	Key                    string              `json:"key"`
	Inputs                 compatibilityInputs `json:"inputs"`
	ObservedContractDigest string              `json:"observed_contract_digest"`
}

func CheckContracts(
	ctx context.Context,
	r buildspec.Resolution,
	p Profile,
	opts buildspec.BuildOptions,
	inputs buildspec.Fingerprints,
	cacheRoot string,
) (digest string, result error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := p.Validate(); err != nil {
		return "", err
	}
	tc := r.Toolchain
	version, err := buildspec.ParseGoVersion(tc.Version)
	if err != nil {
		return "", err
	}
	support, err := buildspec.LookupFamily(version)
	if err != nil {
		return "", err
	}
	if support.Profile != p.ID || !slices.Contains([]string{"linux", "darwin"}, tc.HostOS) ||
		(tc.HostOS == "darwin" && tc.HostArch != "arm64") {
		return "", errors.New("contract profile/platform differs from selected compiler")
	}
	if opts.Experiments != "" || !buildspec.ValidDigest(inputs.ToolchainDigest) ||
		!buildspec.ValidDigest(inputs.StdlibDigest) {
		return "", errors.New("invalid compatibility inputs/experiments")
	}
	for _, tag := range opts.Tags {
		if tag != "sim" && tag != "linkname" {
			return "", fmt.Errorf("unapproved contract build tag %q", tag)
		}
	}
	manifest, err := p.Contracts(tc.HostArch)
	if err != nil {
		return "", err
	}
	if err := p.validateContractHooks(manifest); err != nil {
		return "", err
	}
	digest, err = p.ContractDigest()
	if err != nil {
		return "", err
	}
	dir, cleanup, err := hookInspectionContext(ctx, r)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, cleanup()) }()
	modules, err := contractModuleInputs(ctx, tc, dir, manifest)
	if err != nil {
		return "", err
	}
	keyInputs := compatibilityInputs{
		Schema:          1,
		Family:          support.Family,
		ContractDigest:  digest,
		CheckerRevision: p.Revision,
		Toolchain:       tc.Version,
		Fingerprints:    inputs,
		HostOS:          tc.HostOS,
		Architecture:    tc.HostArch,
		Race:            opts.Race,
		Experiments:     opts.Experiments,
		SourceTags:      buildspec.TranslatedSourceTags(opts.Race),
		Modules:         modules,
	}
	if cacheRoot == "" {
		cacheRoot, err = buildspec.CacheRoot()
		if err != nil {
			return "", err
		}
	}
	err = checkCachedContracts(
		ctx,
		cacheRoot,
		keyInputs,
		func() error { return VerifyContracts(ctx, tc, dir, manifest, opts) },
	)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func contractModuleInputs(
	ctx context.Context,
	tc buildspec.Toolchain,
	dir string,
	m ContractManifest,
) ([]buildspec.Revision, error) {
	var paths []string
	for _, path := range contractPaths(m, false, false) {
		first, _, _ := strings.Cut(path, "/")
		if strings.Contains(first, ".") {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	source, err := inspectSources(ctx, tc, dir, SourceContext{OS: "linux", Arch: m.Architecture}, false, paths)
	if err != nil {
		return nil, err
	}
	var result []buildspec.Revision
	seen := map[string]bool{}
	for _, path := range paths {
		mod := source.packages[path].Module
		if mod == nil {
			return nil, fmt.Errorf("ABI package %s has no source module", path)
		}
		if seen[mod.Path] {
			continue
		}
		seen[mod.Path] = true
		name := mod.Path + "@" + mod.Version
		if mod.Replace != nil {
			mod = mod.Replace
			name += "=>" + mod.Path + "@" + mod.Version
			if mod.Version == "" {
				name = strings.Split(name, "=>")[0] + "=>local"
			}
		}
		digest, err := contractModuleDigest(ctx, mod.Dir)
		if err != nil {
			return nil, err
		}
		result = append(result, buildspec.Revision{Name: name, Digest: digest})
	}
	slices.SortFunc(result, func(a, b buildspec.Revision) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}

func contractModuleDigest(ctx context.Context, root string) (string, error) {
	if root == "" {
		return "", errors.New("ABI module source is unavailable")
	}
	var files []buildspec.Revision
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
		if !entry.Type().IsRegular() {
			return fmt.Errorf("ABI module input is not regular: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, buildspec.Revision{Name: filepath.ToSlash(rel), Digest: buildspec.Hash(data)})
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("empty ABI module source %s", root)
	}
	data, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	return buildspec.Hash(data), nil
}

func checkCachedContracts(
	ctx context.Context,
	root string,
	inputs compatibilityInputs,
	verify func() error,
) (result error) {
	data, err := json.Marshal(inputs)
	if err != nil {
		return err
	}
	key := buildspec.Hash(data)
	want := compatibilityRecord{Schema: 1, Key: key, Inputs: inputs, ObservedContractDigest: inputs.ContractDigest}
	path := filepath.Join(root, "compat", "v1", key+".json")
	hit := func() (bool, error) {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var got compatibilityRecord
		if err := decoder.Decode(&got); err != nil {
			return false, nil
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return false, nil
		}
		return reflect.DeepEqual(got, want), nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if ok, err := hit(); ok || err != nil {
		return err
	}
	unlock, err := buildspec.LockCache(ctx, path+".lock")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, unlock()) }()
	if ok, err := hit(); ok || err != nil {
		return err
	}
	if err := verify(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err = json.Marshal(want)
	if err != nil {
		return err
	}
	stage, err := os.CreateTemp(filepath.Dir(path), ".compat-*")
	if err != nil {
		return err
	}
	defer os.Remove(stage.Name())
	if _, err := stage.Write(append(data, '\n')); err != nil {
		stage.Close()
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(stage.Name(), path)
}
