//go:build go1.25 && !go1.26

package hostabi

import "github.com/neria-cloud/gosim/internal/compat/go125/host"

type Coro struct{ native host.Coro }
