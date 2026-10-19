//go:build go1.25 && !go1.26

package selected

import (
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/go125/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func Profile() spec.Profile { return profile.Descriptor() }

func descriptorForArch(arch string) (spec.Profile, error) { return profile.DescriptorForArch(arch) }
func policyManifest() spec.PolicyManifest                 { return profile.PolicyManifest() }

func Evidence() []buildspec.CertificationReport { return profile.Evidence() }
