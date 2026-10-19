package gosimtool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
)

type BuildConfig struct {
	Context    context.Context
	Resolution buildspec.Resolution
	Identity   buildspec.Identity
	Selection  buildspec.Selection
	Toolchain  buildspec.Toolchain
	Digest     string
}

type TranslateOutput struct {
	RootOutputDir string
	Packages      []string
	Deps          map[string]map[string]string
	Identity      buildspec.Identity
}

// NewBuildConfig takes independent ownership of the validated value records.
// Distribution verification belongs to the build resolver. Translation rechecks
// the worker executable and version instead of trusting the supplied digest.
func NewBuildConfig(ctx context.Context, selection buildspec.Selection, resolution buildspec.Resolution) (BuildConfig, error) {
	p, _, err := selected.Resolve(selection)
	if err != nil {
		return BuildConfig{}, err
	}
	owned, err := p.Select(selection.Identity)
	if err != nil {
		return BuildConfig{}, err
	}
	identity, err := buildspec.NormalizeIdentity(owned.Identity)
	if err != nil {
		return BuildConfig{}, err
	}
	digest, err := buildspec.Digest(identity)
	if err != nil {
		return BuildConfig{}, err
	}
	result := BuildConfig{Context: ctx, Resolution: resolution, Identity: identity, Selection: owned, Toolchain: resolution.Toolchain, Digest: digest}
	return result, result.Validate()
}

func (b BuildConfig) Validate() error {
	if b.Context == nil || b.Resolution.Toolchain != b.Toolchain {
		return fmt.Errorf("missing build context or inconsistent compiler resolution")
	}
	if !slices.Equal(b.Identity.Tags, []string{"linkname", "sim"}) {
		return fmt.Errorf("translation requires the linkname and sim build-policy tags")
	}
	if _, _, err := selected.Resolve(b.Selection); err != nil {
		return err
	}
	if !reflect.DeepEqual(b.Identity, b.Selection.Identity) {
		return fmt.Errorf("build identity differs from immutable selection")
	}
	digest, err := buildspec.Digest(b.Identity)
	if err != nil {
		return err
	}
	if b.Digest != digest {
		return fmt.Errorf("build digest differs from identity")
	}
	tc := b.Toolchain
	if tc.Version != b.Identity.Toolchain || tc.HostOS != b.Identity.HostOS || tc.HostArch != b.Identity.HostArch || !filepath.IsAbs(tc.GOROOT) || tc.Executable != filepath.Join(tc.GOROOT, "bin", "go") {
		return fmt.Errorf("selected toolchain differs from build identity/distribution")
	}
	return nil
}

func (b BuildConfig) AsDirname() string { return b.Digest }

func (b BuildConfig) ModuleGraph() ([]buildspec.Module, error) {
	if b.Resolution.Schema != 1 || !filepath.IsAbs(b.Resolution.Context.Directory) {
		return nil, fmt.Errorf("build requires resolved application module/workspace context")
	}
	modules, err := buildspec.ResolveModuleGraph(b.Context, b.Resolution)
	if err != nil {
		return nil, err
	}
	digest, err := buildcfg.GraphDigest(modules)
	if err != nil {
		return nil, err
	}
	if digest != b.Identity.ModuleGraphDigest {
		return nil, fmt.Errorf("application module/workspace inputs changed after resolution; rerun gosim")
	}
	return modules, nil
}

// ContextFile is also a source dependency of every artifact, including callers
// launched from a workspace root with no nearest go.mod.
func (b BuildConfig) ContextFile() (string, error) {
	for _, path := range []string{b.Resolution.Context.GoMod, b.Resolution.Context.GoWork} {
		if path != os.DevNull && filepath.IsAbs(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("no application go.mod or go.work in resolved context")
}

func (b BuildConfig) ProjectRoot() (string, error) {
	path, err := b.ContextFile()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}
