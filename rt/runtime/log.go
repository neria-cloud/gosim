package gosimruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/neria-cloud/gosim/internal/prettylog"
)

var (
	logLevel      = flag.String("log-level", "ERROR", "gosim slog log level")
	forceChecksum = flag.Bool("force-checksum", false, "gosim force reproducibility checksumming and logging")
)

type logger struct {
	out   io.Writer
	level slog.Level
	slog  *slog.Logger
}

func makeLogger(out io.Writer, level slog.Level) logger {
	ho := slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}
	handler := slog.NewJSONHandler(out, &ho)
	slog := slog.New(wrapHandler{inner: handler})

	return logger{
		out:   out,
		level: level,
		slog:  slog,
	}
}

type wrapHandler struct {
	inner slog.Handler
}

func (w wrapHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return w.inner.Enabled(ctx, level)
}

func (w wrapHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Time = time.Unix(0, Nanotime()).UTC()
	if g := getg(); g != nil {
		r.AddAttrs(slog.String("machine", g.machine.label), slog.Int("goroutine", g.ID))
	}
	return w.inner.Handle(ctx, r)
}

func (w wrapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return wrapHandler{
		inner: w.inner.WithAttrs(attrs),
	}
}

func (w wrapHandler) WithGroup(name string) slog.Handler {
	return wrapHandler{
		inner: w.inner.WithGroup(name),
	}
}

type logformatKind string

const (
	logformatRaw      logformatKind = "raw"
	logformatIndented logformatKind = "indented"
	logformatPretty   logformatKind = "pretty"
)

var logformatFlag logformatKind

func init() {
	logformatFlag = logformatPretty
	flag.Func("logformat", "raw|indented|pretty", func(s string) error {
		k := logformatKind(s)
		if k != logformatRaw && k != logformatIndented && k != logformatPretty {
			return fmt.Errorf("bad log kind %q", s)
		}
		logformatFlag = k
		return nil
	})
}

type indentedWriter struct {
	out io.Writer
}

func (w *indentedWriter) Write(p []byte) (n int, err error) {
	if len(p) > 0 && p[len(p)-1] == '\n' {
		var x any
		if err := json.Unmarshal(p, &x); err == nil {
			o := json.NewEncoder(w.out)
			o.SetIndent("", "  ")
			o.Encode(x)
			return len(p), nil
		}
	}
	w.out.Write(p)
	return len(p), nil
}

func makeConsoleLogger(out io.Writer) io.Writer {
	switch logformatFlag {
	case logformatRaw:
		return out
	case logformatIndented:
		return &indentedWriter{
			out: out,
		}
	case logformatPretty:
		return prettylog.NewWriter(out)
	default:
		panic(logformatFlag)
	}
}

// consoleOutput is owned by the native CLI process. Metatest/API runs leave it
// nil and return captured output only; it never carries translated values.
var consoleOutput io.Writer

type consoleWriter struct{ out, formatted io.Writer }

func (w *consoleWriter) Write(p []byte) (int, error) {
	for _, line := range bytes.SplitAfter(p, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		output := w.out
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("{")) {
			output = w.formatted
		}
		if _, err := output.Write(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
