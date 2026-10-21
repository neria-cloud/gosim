// Package testutil contains shared test fixtures, never runtime admission rules.
package testutil

import (
	"fmt"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// UnregisteredGo follows the registry so onboarding does not invalidate negative tests.
func UnregisteredGo() (v struct {
	Version, Family, Profile string
	Minor                    int
},
) {
	for _, f := range buildspec.SupportedFamilies() {
		parsed, err := buildspec.ParseGoVersion("go" + f.Family + ".0")
		if err != nil {
			panic(err)
		}
		v.Minor = max(v.Minor, parsed.Minor+1)
	}
	v.Family = fmt.Sprintf("1.%d", v.Minor)
	v.Profile = fmt.Sprintf("go1%d", v.Minor)
	v.Version = "go" + v.Family + ".1"
	return v
}
