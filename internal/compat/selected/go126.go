//go:build go1.26 && !go1.27

package selected

import (
	"github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func Profile() spec.Profile { return profile.Descriptor() }

func descriptorForArch(arch string) (spec.Profile, error) { return profile.DescriptorForArch(arch) }
func policyManifest() spec.PolicyManifest                 { return profile.PolicyManifest() }
