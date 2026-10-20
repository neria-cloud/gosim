//go:build go1.25 && !go1.26

// Package signal keeps imported signal APIs inside the simulation boundary.
// Every operation fails closed: simulated process signals are not implemented.
package signal

import (
	"context"
	"os"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported() {
	gosimruntime.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "rt",
			Operation: "os/signal",
			Message:   "simulated process signals are not implemented",
		},
	)
}

func Notify(_ chan<- os.Signal, sig ...os.Signal) { unsupported() }
func Stop(_ chan<- os.Signal)                     { unsupported() }
func Reset(_ ...os.Signal)                        { unsupported() }
func Ignore(_ ...os.Signal)                       { unsupported() }
func Ignored(_ os.Signal) bool                    { unsupported(); return false }
func NotifyContext(_ context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	unsupported()
	panic("abort returned")
}
