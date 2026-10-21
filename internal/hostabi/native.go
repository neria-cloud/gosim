//go:build go1.25

package hostabi

import "github.com/neria-cloud/gosim/internal/compat/host"

type Coro struct{ native host.Coro }
