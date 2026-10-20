package gosimcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

var (
	simulationRun    = regexp.MustCompile(`^(\s*=== RUN\s+)(.+) \(seed (-?\d+)\)$`)
	simulationResult = regexp.MustCompile(`^(\s*--- (?:PASS|FAIL|SKIP): )(.+) \([0-9.]+s simulated\)$`)
)

// Gosim's human-readable headers contain a seed and simulated duration. The Go
// converter needs matching names and real elapsed seconds. Preserve the seed
// and original result line as test output, and time execution on the host.
type simulationOutput struct {
	out     io.Writer
	pending []byte
	starts  map[string]time.Time
}

func (w *simulationOutput) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		end := bytes.IndexByte(w.pending, '\n')
		if end < 0 {
			break
		}
		line := string(w.pending[:end])
		w.pending = w.pending[end+1:]
		if err := w.line(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *simulationOutput) line(line string) error {
	if match := simulationRun.FindStringSubmatch(line); match != nil {
		w.starts[match[2]] = time.Now()
		_, err := fmt.Fprintf(w.out, "%s%s\n    seed: %s\n", strings.TrimLeft(match[1], " \t"), match[2], match[3])
		return err
	}
	if match := simulationResult.FindStringSubmatch(line); match != nil {
		elapsed := 0.0
		if start, ok := w.starts[match[2]]; ok {
			elapsed = time.Since(start).Seconds()
			delete(w.starts, match[2])
		}
		_, err := fmt.Fprintf(
			w.out,
			"    simulated result: %s\n%s%s (%.6fs)\n",
			strings.TrimSpace(line),
			strings.TrimLeft(match[1], " \t"),
			match[2],
			elapsed,
		)
		return err
	}
	_, err := fmt.Fprintln(w.out, line)
	return err
}

func captureSimulationCommand(
	ctx context.Context,
	binary, converter *exec.Cmd,
	mu *sync.Mutex,
	stream io.Writer,
) ([]byte, error) {
	input, output := io.Pipe()
	converter.Stdin = input
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := captureCommand(ctx, converter, mu, stream)
		// Unblock the binary writer if the converter fails to start or exits early.
		_ = input.CloseWithError(io.ErrClosedPipe)
		done <- result{data, err}
	}()
	normalized := &simulationOutput{out: output, starts: make(map[string]time.Time)}
	binary.Stdout = normalized
	binary.Stderr = normalized
	runErr := buildspec.RunProcess(ctx, binary)
	var flushErr error
	if len(normalized.pending) > 0 {
		flushErr = normalized.line(string(normalized.pending))
	}
	status := "PASS"
	if runErr != nil || flushErr != nil {
		status = "FAIL"
	}
	// Gosim exits with the outcome but does not print Go's package result marker.
	// test2json in stdin mode needs that marker to close the package accurately.
	_, writeErr := fmt.Fprintln(output, status)
	_ = output.Close()
	converted := <-done
	return converted.data, errors.Join(runErr, flushErr, writeErr, converted.err)
}
