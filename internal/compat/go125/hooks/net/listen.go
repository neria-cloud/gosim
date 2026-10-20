//go:build go1.25 && !go1.26

package net

import (
	kh "github.com/neria-cloud/gosim/internal/compat/go125/hooks/kernel"
	"github.com/neria-cloud/gosim/internal/contracts"
)

// Net_maxListenerBacklog uses the model's bounded accept queue. It never reads
// host or simulated proc files, so network listeners do not require fs.
func Net_maxListenerBacklog() int {
	kh.Require(contracts.Net, "net.maxListenerBacklog")
	return contracts.ListenerBacklog
}
