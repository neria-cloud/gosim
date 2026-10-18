package network

import (
	"log/slog"

	"github.com/neria-cloud/gosim/gosimruntime"
)

// traceNetwork never inspects payloads or consumes randomness.
func traceNetwork(event string, attrs ...any) {
	if !gosimruntime.TraceNetwork.Enabled() {
		return
	}
	attrs = append(attrs, "traceKind", "network")
	slog.Info("network."+event, attrs...)
}
