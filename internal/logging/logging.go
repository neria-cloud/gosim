// Package logging installs the translated, per-machine structured log handler.
// Only generated composition calls Setup, after application globals initialize.
package logging

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/neria-cloud/gosim/internal/gosimlog"
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

type gosimSlogHandler struct {
	inner slog.Handler
}

func (w gosimSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return w.inner.Enabled(ctx, level)
}

func (w gosimSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.Int("goroutine", gosimruntime.GetGoroutine()))
	hasStep := false
	for attr := range r.Attrs {
		if attr.Key == "step" {
			hasStep = true
		}
	}
	if !hasStep {
		r.AddAttrs(slog.Int("step", gosimruntime.Step()))
	}
	if gosimruntime.TraceStack.Enabled() {
		r.AddAttrs(gosimlog.Stack(0, r.PC))
	}
	return w.inner.Handle(ctx, r)
}

func (w gosimSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return gosimSlogHandler{
		inner: w.inner.WithAttrs(attrs),
	}
}

func (w gosimSlogHandler) WithGroup(name string) slog.Handler {
	return gosimSlogHandler{
		inner: w.inner.WithGroup(name),
	}
}

type gosimLogWriter struct{}

func (w gosimLogWriter) Write(b []byte) (n int, err error) {
	gosimruntime.WriteLog(b)
	return len(b), nil
}

func makeBaseSlogHandler() slog.Handler {
	var level slog.Level
	if value := os.Getenv("GOSIM_LOG_LEVEL"); value != "" {
		if err := level.UnmarshalText([]byte(value)); err != nil {
			panic(err)
		}
	}

	ho := slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}
	return slog.NewJSONHandler(gosimLogWriter{}, &ho)
}

func Setup(machineLabel string) {
	// We play a funny game with the logger. There exists a default slog.Logger in
	// every machine, since they all have their own set of globals. All loggers
	// point to the same LogOut writer and are set up using the configuration set
	// here.

	// TODO racedetector: make sure that using slog doesn't introduce sneaky
	// happens-before (this currently happens in the default handler which
	// uses both a pool and a lock)

	// stdout and stderr are currently captured with some special logic in
	// LinuxOS for writes to their file descriptors, see TestStdoutStderr.

	handler := makeBaseSlogHandler()

	// set short file flag so that we'll capture source info. see slog.SetDefault internals
	// XXX: test this?
	log.SetFlags(log.Lshortfile)
	slog.SetDefault(slog.New(gosimSlogHandler{inner: handler}).With("machine", machineLabel))
}
