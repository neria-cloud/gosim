// Package catalog exposes generation metadata without importing executable adapters.
package catalog

import (
	"fmt"

	"github.com/neria-cloud/gosim/internal/buildspec"
	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

type Entry struct {
	Descriptor func(string) (spec.Profile, error)
	Origin     func() spec.Origin
	Policy     func() spec.PolicyManifest
	Shims      func() []spec.Hook
}

var entries = map[string]Entry{
	"go125": {p125.DescriptorForArch, p125.VerificationOrigin, p125.PolicyManifest, p125.ShimManifest},
	"go126": {p126.DescriptorForArch, p126.VerificationOrigin, p126.PolicyManifest, p126.ShimManifest},
}

func Lookup(profile string) (Entry, error) {
	if _, err := buildspec.LookupProfile(profile); err != nil {
		return Entry{}, err
	}
	entry, ok := entries[profile]
	if !ok {
		return Entry{}, fmt.Errorf("registered profile %s has no implemented metadata", profile)
	}
	return entry, nil
}

func Descriptor(family, arch string) (spec.Profile, error) {
	for _, support := range buildspec.SupportedFamilies() {
		if support.Family != family {
			continue
		}
		entry, err := Lookup(support.Profile)
		if err != nil {
			return spec.Profile{}, err
		}
		return entry.Descriptor(arch)
	}
	return spec.Profile{}, fmt.Errorf("unsupported Go family %s", family)
}
