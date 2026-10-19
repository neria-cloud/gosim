package gosimtool

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
)

type BuildConfig struct {
	Identity  buildspec.Identity
	Selection buildspec.Selection
	Toolchain buildspec.Toolchain
	Digest    string
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
func NewBuildConfig(selection buildspec.Selection, tc buildspec.Toolchain) (BuildConfig, error) {
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
	result := BuildConfig{Identity: identity, Selection: owned, Toolchain: tc, Digest: digest}
	return result, result.Validate()
}

func (b BuildConfig) Validate() error {
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
