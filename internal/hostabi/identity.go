//go:build go1.25 && !go1.27

package hostabi

import (
	"runtime"

	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// Identity reports the compiled adapter source, not a compatibility claim.
// runtime.Version is diagnostic only; source selection uses bounded build tags.
func Identity() abi.HostIdentity {
	p := selected.Profile()
	return abi.HostIdentity{Toolchain: runtime.Version(), Profile: p.ID, HostOS: runtime.GOOS, HostArch: runtime.GOARCH, SourceDigest: p.Revision}
}
