//go:build go1.26 && !go1.27

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
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "os/signal", Message: "simulated process signals are not implemented"})
}

func Notify(c chan<- os.Signal, sig ...os.Signal) { unsupported() }
func Stop(c chan<- os.Signal)                     { unsupported() }
func Reset(sig ...os.Signal)                      { unsupported() }
func Ignore(sig ...os.Signal)                     { unsupported() }
func Ignored(sig os.Signal) bool                  { unsupported(); return false }
func NotifyContext(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	unsupported()
	panic("abort returned")
}
