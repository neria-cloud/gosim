//go:build go1.25 && !go1.26

package selected

import (
	"github.com/neria-cloud/gosim/internal/compat/go125/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func Profile() spec.Profile { return profile.Descriptor() }
