package selected

import (
	"fmt"
	"reflect"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

// Resolve checks values at the host-tool boundary and returns independently
// owned metadata. Executable adapters are selected by Go build constraints;
// supplying another family's identity cannot switch them at runtime.
func Resolve(selection buildspec.Selection) (spec.Profile, spec.PolicyManifest, error) {
	p, err := descriptorForArch(selection.Identity.SimulationArch)
	if err != nil {
		return spec.Profile{}, spec.PolicyManifest{}, err
	}
	expected, err := p.Select(selection.Identity)
	if err != nil {
		return spec.Profile{}, spec.PolicyManifest{}, err
	}
	if !reflect.DeepEqual(expected, selection) {
		return spec.Profile{}, spec.PolicyManifest{}, fmt.Errorf(
			"selection differs from canonical %s profile metadata",
			p.ID,
		)
	}
	m := policyManifest()
	if err := m.Validate(p); err != nil {
		return spec.Profile{}, spec.PolicyManifest{}, err
	}
	return p, m, nil
}
