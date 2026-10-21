// Package profiles exposes cumulative metadata without executable adapters or provenance embeds.
package profiles

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

//go:embed go*[0-9].json
var metadata embed.FS

var definitions = sync.OnceValues(func() (map[string]spec.FamilyDefinition, error) {
	return spec.LoadDefinitions(metadata)
})

// Definitions returns independent nested records; the lazy cache is immutable.
func Definitions() (map[string]spec.FamilyDefinition, error) {
	defs, err := definitions()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(defs)
	if err != nil {
		return nil, err
	}
	var out map[string]spec.FamilyDefinition
	err = json.Unmarshal(data, &out)
	return out, err
}

func Resolve(family string) (spec.ResolvedFamily, error) {
	defs, err := definitions()
	if err != nil {
		return spec.ResolvedFamily{}, err
	}
	return spec.ResolveDefinitions(defs, family)
}

func supported(profileID string) (spec.ResolvedFamily, error) {
	support, err := buildspec.LookupProfile(profileID)
	if err != nil {
		return spec.ResolvedFamily{}, err
	}
	r, err := Resolve(support.Family)
	if err != nil {
		return r, err
	}
	if !r.Supported || r.ProfileID != support.Profile {
		return spec.ResolvedFamily{}, fmt.Errorf("profile %s disagrees with support registry", profileID)
	}
	return r, nil
}

func Descriptor(profileID, arch string) (spec.Profile, error) {
	r, err := supported(profileID)
	if err != nil {
		return spec.Profile{}, err
	}
	revision, err := Revision(profileID)
	if err != nil {
		return spec.Profile{}, err
	}
	p, err := r.Descriptor(arch, revision)
	if err != nil {
		return p, err
	}
	return p, p.Validate()
}

func Policy(profileID string) (spec.PolicyManifest, error) {
	r, err := supported(profileID)
	if err != nil {
		return spec.PolicyManifest{}, err
	}
	return r.PolicyManifest(), nil
}

func Shims(profileID, arch string) ([]spec.Hook, error) {
	r, err := supported(profileID)
	if err != nil {
		return nil, err
	}
	if arch != "amd64" && arch != "arm64" {
		return nil, fmt.Errorf("unsupported simulation architecture %q", arch)
	}
	return r.ShimManifest(arch), nil
}

func Revision(profileID string) (string, error) {
	if _, err := supported(profileID); err != nil {
		return "", err
	}
	revision := revisions[profileID]
	if !buildspec.ValidDigest(revision) {
		return "", fmt.Errorf("missing/invalid revision for %s", profileID)
	}
	return revision, nil
}
