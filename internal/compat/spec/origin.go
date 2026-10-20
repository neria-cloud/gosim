package spec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// Origin records the exact upstream files reviewed for private ABI access. It is
// source provenance, not caller compatibility or execution evidence.
// Ordinary commands check private contracts against their own resolved compiler.
type Origin struct {
	Schema            int          `json:"schema"`
	Family            string       `json:"family"`
	BaselineToolchain string       `json:"baseline_toolchain"`
	Files             []SourceFile `json:"files"`
}

type SourceFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	// Native declarations include race-only files that VerifyHooks' Linux/non-race
	// source selection intentionally does not load.
	Hooks []Hook `json:"hooks,omitempty"`
}

func VerifyBaselineOrigin(tc buildspec.Toolchain, origin Origin) error {
	version, err := buildspec.ParseGoVersion(origin.BaselineToolchain)
	if err != nil || origin.Schema != 2 || version.Family() != origin.Family {
		return fmt.Errorf("invalid baseline origin metadata")
	}
	if tc.Version != origin.BaselineToolchain || len(origin.Files) == 0 {
		return fmt.Errorf("native ABI origin requires %s, got %s", origin.BaselineToolchain, tc.Version)
	}
	output, err := buildspec.GoCommand(tc, "", "version").Output()
	if err != nil || !strings.HasPrefix(string(output), "go version "+origin.BaselineToolchain+" ") {
		return fmt.Errorf("native ABI executable does not match %s: %s (%v)", origin.BaselineToolchain, output, err)
	}
	versionFile, err := os.ReadFile(filepath.Join(tc.GOROOT, "VERSION"))
	if err != nil || strings.SplitN(string(versionFile), "\n", 2)[0] != origin.BaselineToolchain {
		return fmt.Errorf("native ABI GOROOT does not match %s", origin.BaselineToolchain)
	}
	seen := make(map[string]bool)
	for _, file := range origin.Files {
		if !strings.HasPrefix(file.Path, "src/") || filepath.ToSlash(filepath.Clean(file.Path)) != file.Path ||
			strings.Contains(file.Path, "\\") ||
			seen[file.Path] ||
			!buildspec.ValidDigest(file.Digest) {
			return fmt.Errorf("invalid native ABI source %q", file.Path)
		}
		seen[file.Path] = true
		filename := filepath.Join(tc.GOROOT, filepath.FromSlash(file.Path))
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("native ABI source %s: %w", file.Path, err)
		}
		if buildspec.Hash(data) != file.Digest {
			return fmt.Errorf("native ABI source modified: %s", file.Path)
		}
		for _, hook := range file.Hooks {
			symbols := make(map[string]string)
			if err := readHookDeclarations(filename, hook.Package, symbols); err != nil {
				return err
			}
			if got := symbols[hook.Package+"."+hook.Symbol]; got != hook.Signature {
				return fmt.Errorf(
					"native ABI declaration drift for %s.%s: got %q, expected %q",
					hook.Package,
					hook.Symbol,
					got,
					hook.Signature,
				)
			}
		}
	}
	return nil
}

// RefreshOrigin updates provenance only after the generator checks reviewed contracts.
// It does not approve new hook/layout expectations or change family eligibility.
func RefreshOrigin(
	ctx context.Context,
	r buildspec.Resolution,
	previous Origin,
	manifests []ContractManifest,
) (result Origin, resultErr error) {
	tc := r.Toolchain
	v, err := buildspec.ParseGoVersion(tc.Version)
	if err != nil || v.Family() != previous.Family {
		return Origin{}, fmt.Errorf("origin refresh requires the same Go family")
	}
	if len(manifests) == 0 {
		return Origin{}, fmt.Errorf("origin refresh requires reviewed contracts")
	}
	dir, cleanup, err := hookInspectionContext(ctx, r)
	if err != nil {
		return Origin{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, cleanup()) }()
	files := map[string]SourceFile{}
	// Keep still-present reviewed inputs; moved/removed files are replaced by the
	// actual selected source inventory below. Compatibility never depends on names.
	for _, file := range previous.Files {
		if !strings.HasPrefix(file.Path, "src/") || filepath.ToSlash(filepath.Clean(file.Path)) != file.Path ||
			strings.Contains(file.Path, "\\") ||
			files[file.Path].Path != "" {
			return Origin{}, fmt.Errorf("invalid origin path %q", file.Path)
		}
		if _, err := os.Stat(filepath.Join(tc.GOROOT, filepath.FromSlash(file.Path))); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return Origin{}, err
		}
		files[file.Path] = SourceFile{Path: file.Path}
	}
	for _, manifest := range manifests {
		if err := manifest.Validate(); err != nil {
			return Origin{}, err
		}
		if manifest.Family != previous.Family {
			return Origin{}, fmt.Errorf("origin contract family mismatch")
		}
		for _, race := range []bool{false, true} {
			if err := VerifyContracts(ctx, tc, dir, manifest, buildspec.BuildOptions{Race: race}); err != nil {
				return Origin{}, err
			}
		}
		for _, native := range []bool{false, true} {
			for _, race := range []bool{false, true} {
				selection := SourceContext{OS: "linux", Arch: manifest.Architecture, Race: race, Native: native}
				if native {
					selection.OS = tc.HostOS
					selection.Arch = tc.HostArch
				}
				paths := contractPaths(manifest, native, race)
				if len(paths) == 0 {
					continue
				}
				sources, err := inspectSources(ctx, tc, dir, selection, false, paths)
				if err != nil {
					return Origin{}, err
				}
				for _, pkg := range sources.packages {
					rel, err := filepath.Rel(tc.GOROOT, pkg.Dir)
					if err != nil || !strings.HasPrefix(filepath.ToSlash(rel), "src/") {
						continue
					}
					for _, name := range pkg.GoFiles {
						path := filepath.ToSlash(filepath.Join(rel, name))
						record := files[path]
						record.Path = path
						if native {
							symbols := map[string]string{}
							if err := readHookDeclarations(filepath.Join(pkg.Dir, name), pkg.ImportPath, symbols); err != nil {
								return Origin{}, err
							}
							for _, hook := range manifest.Hooks {
								if hook.Context == "" || hook.Package != pkg.ImportPath ||
									(hook.Context == "host-race" && !race) {
									continue
								}
								if signature, found := symbols[hook.Package+"."+hook.Symbol]; found {
									if signature != hook.Signature {
										return Origin{}, fmt.Errorf(
											"native ABI declaration drift for %s.%s",
											hook.Package,
											hook.Symbol,
										)
									}
									hook.Context = ""
									if !slices.Contains(record.Hooks, hook) {
										record.Hooks = append(record.Hooks, hook)
									}
								}
							}
						}
						files[path] = record
					}
				}
			}
		}
	}
	result = Origin{Schema: 2, Family: previous.Family, BaselineToolchain: tc.Version}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(tc.GOROOT, filepath.FromSlash(file.Path)))
		if err != nil {
			return Origin{}, err
		}
		file.Digest = buildspec.Hash(data)
		slices.SortFunc(
			file.Hooks,
			func(a, b Hook) int { return strings.Compare(a.Package+"."+a.Symbol, b.Package+"."+b.Symbol) },
		)
		result.Files = append(result.Files, file)
	}
	slices.SortFunc(result.Files, func(a, b SourceFile) int { return strings.Compare(a.Path, b.Path) })
	if err := VerifyBaselineOrigin(tc, result); err != nil {
		return Origin{}, err
	}
	return result, nil
}
