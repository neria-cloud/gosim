//go:build go1.25

// Package trace models tracing as disabled for simulated application code.
// Active runtime tracing and annotations require scheduler and output semantics
// which are not supported here.
package trace

import (
	"context"
	"io"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported(operation string) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime/trace." + operation, Message: "active runtime tracing is not modeled"})
}

// IsEnabled is the sole positive operation. It prevents OpenTelemetry's
// optional task branch from publishing trace state in a simulation.
func IsEnabled() bool { return false }

type (
	Task   struct{}
	Region struct{}
)

// TODO(not planned): active tracing needs machine-scoped task identity,
// region lifecycle and a deterministic trace output stream.
func Start(io.Writer) error {
	unsupported("Start")
	panic("abort returned")
}
func Stop() { unsupported("Stop") }
func NewTask(context.Context, string) (context.Context, *Task) {
	unsupported("NewTask")
	panic("abort returned")
}
func (t *Task) End() { unsupported("Task.End") }
func StartRegion(context.Context, string) *Region {
	unsupported("StartRegion")
	panic("abort returned")
}
func (r *Region) End()                             { unsupported("Region.End") }
func WithRegion(context.Context, string, func())   { unsupported("WithRegion") }
func Log(context.Context, string, string)          { unsupported("Log") }
func Logf(context.Context, string, string, ...any) { unsupported("Logf") }
