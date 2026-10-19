package gosimruntime

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"slices"

	"github.com/neria-cloud/gosim/internal/hostabi"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// HostIdentity exposes only the primitive native identity record. Generated
// modules cannot import the original private host ABI facade directly.
func HostIdentity() abi.HostIdentity { return hostabi.Identity() }

type runCapture struct {
	config    RunConfig
	log       bytes.Buffer
	checksum  []byte
	finalized bool
}

// ConfigureRun installs copied runner inputs before entering Engine.Run. Trace
// flags and scheduler state are applied only after the single-run guard is held.
func (e *Engine) ConfigureRun(config RunConfig) error {
	if e.ran || e.running || e.closed || e.capture != nil || config.Seed != e.config.Seed {
		return ErrEngineState
	}
	config.ExtraEnv = slices.Clone(config.ExtraEnv)
	e.capture = &runCapture{config: config}
	return nil
}

func (e *Engine) startCapture() (func(), error) {
	c := e.capture
	if c == nil {
		return func() {}, nil
	}
	previous := make(map[*TraceFlag]bool, len(traceflags))
	for _, flag := range traceflags {
		previous[flag] = flag.Enabled()
	}
	restore := func() {
		for flag, enabled := range previous {
			flag.set(enabled)
		}
	}
	if err := parseTraceflagsConfig(c.config.Simtrace); err != nil {
		restore()
		return nil, err
	}
	logger := makeLogger(&c.log, slog.LevelInfo)
	checksum := newChecksummer(logger.slog)
	logger = makeLogger(io.MultiWriter(&checksumWriter{checksummer: checksum}, &c.log), slog.LevelInfo)
	e.scheduler.logger = logger
	e.scheduler.checksummer = checksum
	e.scheduler.envs = append(slices.Clone(c.config.ExtraEnv), "GOSIM_LOG_LEVEL=INFO")
	checksum.recordIntInt(checksumKeyRunStarted, 0, 0)
	return restore, nil
}

// RunResult returns owned primitive data after ordered kernel and engine cleanup.
// failure is already formatted inside the translated supervisor: translated
// error implementations must never run after their globals have been disposed.
func (e *Engine) RunResult(buildDigest, failure string) RunResult {
	result := RunResult{Seed: e.config.Seed, BuildDigest: buildDigest, ReplayRevision: e.config.ReplayRevision}
	if !e.closed || e.running || e.capture == nil {
		result.Failed = true
		result.Err = fmt.Sprintf("%v: result requested before configured engine cleanup", ErrEngineState)
		return result
	}
	if failure == "" && e.result != nil {
		failure = e.result.Error()
	}
	if failure == "" && !e.ran {
		failure = ErrEngineState.Error()
	}
	c := e.capture
	if !c.finalized {
		if checksum := e.scheduler.checksummer; checksum != nil {
			checksum.recordIntInt(checksumKeyRunFinished, 0, 0)
			c.checksum = checksum.finalize()
		}
		c.finalized = true
	}
	result.Checksum = slices.Clone(c.checksum)
	result.LogOutput = slices.Clone(c.log.Bytes())
	result.Failed = failure != ""
	result.Err = failure
	return result
}
