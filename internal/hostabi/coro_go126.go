//go:build go1.26 && !go1.27

package hostabi

import "github.com/neria-cloud/gosim/internal/compat/go126/host"

type Coro struct{ native host.Coro }
