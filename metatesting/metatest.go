package metatesting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/prettylog"
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

const metatestProtocol = 3

// The structs below are copied from gosimruntime.runConfig and
// gosimruntime.runResult. Keep in sync.
// They are copied to give a nice documentation view without refering to
// internal types.

// A RunConfig configures a test invocation.
type RunConfig struct {
	Test     string
	Seed     int64
	ExtraEnv []string
	Simtrace string
}

// A RunResult contains the result fo a test run.
type RunResult struct {
	Seed           int64
	Checksum       []byte
	Failed         bool
	LogOutput      []byte
	Err            string // TODO: reconsider this type?
	BuildDigest    string
	ReplayRevision string
}

// A MetaT runs gosim tests for a given package. It can run tests with various
// flags and returns their results.
//
// Behind the scenes a MetaT executes and controls a gosim test binary.
type (
	Options  struct{ ConfigPath string }
	Manifest = gosimtool.Manifest
)

type MetaT struct {
	cmd       *exec.Cmd
	mu        sync.Mutex
	stdin     io.Closer
	stdout    io.Closer
	failedRun atomic.Bool
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
	expected  gosimruntime.MetatestHandshake
	w         *json.Encoder
	r         *json.Decoder
}

type prefixer struct {
	out    *os.File
	prefix string
}

func (w prefixer) Write(b []byte) (n int, err error) {
	n = len(b)
	remaining := b
	for len(remaining) > 0 {
		end := bytes.IndexByte(remaining, '\n')
		if end == -1 {
			end = len(remaining)
		} else {
			end++
		}
		line := remaining[:end]
		_, _ = w.out.WriteString(w.prefix)
		_, _ = w.out.Write(line)
		if line[len(line)-1] != '\n' {
			_, _ = w.out.WriteString("\n")
		}
		remaining = remaining[end:]
	}
	return
}

func newRunner(path string, expected Manifest) (*MetaT, error) {
	return startRunner(path, expected, 5*time.Second)
}

func startRunner(path string, expected Manifest, startupTimeout time.Duration) (*MetaT, error) {
	digest, err := buildspec.Digest(expected.Identity)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if buildspec.Hash(data) != expected.Binary.SHA256 {
		return nil, fmt.Errorf("metatest binary content mismatch")
	}
	cmd := exec.Command(path, "-metatest")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	cmd.Stderr = &prefixer{out: os.Stderr, prefix: "metatest binary: "}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	runner := &MetaT{
		cmd: cmd, stdin: stdin, stdout: stdout,
		w: json.NewEncoder(stdin), r: json.NewDecoder(stdout),
		expected: gosimruntime.MetatestHandshake{
			Protocol: metatestProtocol, IdentityDigest: digest, Toolchain: expected.Identity.Toolchain,
		},
	}
	ready := make(chan error, 1)
	go func() {
		var hello gosimruntime.MetatestHandshake
		if err := runner.r.Decode(&hello); err != nil {
			ready <- err
			return
		}
		if hello != runner.expected {
			ready <- fmt.Errorf("metatest protocol/identity mismatch: got %+v, expected %+v", hello, runner.expected)
			return
		}
		ready <- runner.w.Encode(runner.expected)
	}()
	timer := time.NewTimer(startupTimeout)
	defer timer.Stop()
	timedOut := false
	select {
	case err = <-ready:
	case <-timer.C:
		timedOut = true
		err = fmt.Errorf("metatest handshake timeout")
	}
	if err != nil {
		cleanup := runner.Close()
		if timedOut {
			<-ready // closing the pipes joins the owned handshake goroutine
		}
		return nil, errors.Join(err, cleanup)
	}
	return runner, nil
}

// Close ends this runner's request stream and joins its process and stderr
// copier. It also interrupts a blocked request; it never waits for the request
// mutex before closing pipes. Concurrent and repeated calls return one result.
func (r *MetaT) Close() error {
	r.closeOnce.Do(func() {
		r.closed.Store(true)
		closeErr := r.stdin.Close()
		if errors.Is(closeErr, os.ErrClosed) {
			closeErr = nil
		}
		_ = r.stdout.Close()
		timer := time.AfterFunc(5*time.Second, func() { _ = r.cmd.Process.Kill() })
		waitErr := r.cmd.Wait()
		timer.Stop()
		var exitErr *exec.ExitError
		if r.failedRun.Load() && errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 66 {
			waitErr = nil
		}
		r.closeErr = errors.Join(closeErr, waitErr)
	})
	return r.closeErr
}

func (r *MetaT) protocolFailure(err error) error {
	return errors.Join(err, r.Close())
}

// TODO: Support match expressions?
func (r *MetaT) ListTests() ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return nil, fmt.Errorf("metatest runner is closed")
	}
	if err := r.w.Encode(&RunConfig{
		Test: "listtests",
	}); err != nil {
		return nil, r.protocolFailure(err)
	}

	var tests []string
	if err := r.r.Decode(&tests); err != nil {
		return nil, r.protocolFailure(err)
	}

	return tests, nil
}

func (r *MetaT) RunAllTests(t *testing.T) {
	// TODO: Parallel tests for multiple seeds/tests?

	tests, err := r.ListTests()
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			run, err := r.Run(t, &RunConfig{
				Test: test,
				Seed: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if run.Failed {
				t.Error("failed")
			}
		})
	}
}

func (r *MetaT) Run(t *testing.T, config *RunConfig) (*RunResult, error) {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return nil, fmt.Errorf("metatest runner is closed")
	}
	// A broken stream closes and reaps the child before returning.

	b := new(bytes.Buffer)

	// TODO: include "gosim test ..." line?
	fmt.Fprintf(b, "\n> running %s [seed=%d]\n", config.Test, config.Seed)

	if err := r.w.Encode(config); err != nil {
		return nil, r.protocolFailure(err)
	}

	var result RunResult
	if err := r.r.Decode(&result); err != nil {
		// TODO: Print results that fail to decode
		return nil, r.protocolFailure(err)
	}

	if result.BuildDigest != r.expected.IdentityDigest || result.ReplayRevision != gosimruntime.ReplayRevision {
		return nil, r.protocolFailure(fmt.Errorf("metatest result identity mismatch"))
	}
	if result.Failed {
		r.failedRun.Store(true)
	}
	w := prettylog.NewWriter(b)

	out := result.LogOutput
	for len(out) > 0 {
		idx := bytes.IndexByte(out, '\n')
		if idx == -1 {
			idx = len(out) - 1
		}
		line := out[:idx+1]
		out = out[idx+1:]
		_, _ = w.Write([]byte(line))
	}
	t.Log(b.String())

	return &result, nil
}

// ForOtherPackage owns a fresh worker and registers its cleanup with t.
func ForOtherPackage(t *testing.T, pkg string, opts Options) *MetaT {
	t.Helper()
	if gosimruntime.IsSim() {
		t.Fatalf("metatest cannot be used from within gosim")
	}
	identity := gosimtool.ResolveTestIdentity(t, opts.ConfigPath)
	path := gosimtool.GetPathForPrecompiledTestBinary(t, pkg, identity)
	manifest, err := gosimtool.ManifestForBinary(path, identity)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := newRunner(path, manifest)
	if err != nil {
		t.Fatalf("failed to run test binary: %s", err)
	}
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Errorf("metatest worker cleanup: %v", err)
		}
	})
	return runner
}

// getCurrentPackageFromWorkingdir guesses the package
// that `go test` is testing from the working directory.
func getCurrentPackageFromWorkingdir() string {
	// go test runs all tests with the working directory set to
	// the current package's source code. Walk upwards to find
	// the go.mod file, and then combine the module path
	// with the relative path to the working directory.
	goModPath, modFile, err := gosimtool.FindGoMod()
	if err != nil {
		stdlog.Fatal(err)
	}
	goModDir := filepath.Dir(goModPath)
	baseDir, err := os.Getwd()
	if err != nil {
		stdlog.Fatal(err)
	}
	relativePkg := strings.TrimPrefix(baseDir, goModDir)
	finalPkg := modFile.Module.Mod.Path + relativePkg
	return finalPkg
}

// ForPackage returns a *MetaT for the package currently
// being tested by `go test`.
//
// ForPackage relies on the working directory to identify the current
// package, so if a test changes directories ForPackage will not work
// correctly.
func ForPackage(t *testing.T, opts Options) *MetaT {
	if gosimruntime.IsSim() {
		t.Fatalf("metatest cannot be used from within gosim")
	}

	return ForOtherPackage(t, getCurrentPackageFromWorkingdir(), opts)
}

func CheckDeterministic(t *testing.T, mt *MetaT) {
	tests, err := mt.ListTests()
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			run1, err := mt.Run(t, &RunConfig{
				Test: test,
				Seed: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			run2, err := mt.Run(t, &RunConfig{
				Test: test,
				Seed: 1,
			})
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(run1.Checksum, run2.Checksum) {
				slog.Error("checksums differ: non-determinism found")
				t.Fail()
				// XXX debug view?
			}
			if !bytes.Equal(run1.LogOutput, run2.LogOutput) {
				slog.Error("logs differ: non-determinism found", "diff", cmp.Diff(run1.LogOutput, run2.LogOutput))
				t.Fail()
			}
		})
	}
}

func CheckSeeds(t *testing.T, mt *MetaT, numSeeds int) {
	tests, err := mt.ListTests()
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			for seed := int64(0); seed < int64(numSeeds); seed++ {
				run, err := mt.Run(t, &RunConfig{
					Test: test,
					Seed: seed,
				})
				if err != nil {
					t.Fatal(err)
				}
				if run.Failed {
					t.Error("failed")
				}
				// XXX: how do we assert run success?
			}
		})
	}
}

func ParseLog(logs []byte) []map[string]any {
	var out []map[string]any

	for _, line := range bytes.Split(logs, []byte("\n")) {
		var log map[string]any
		if err := json.Unmarshal(line, &log); err != nil {
			// TODO: Is this ok?
			continue
		}
		out = append(out, log)
	}

	return out
}

func SimplifyParsedLog(logs []map[string]any) []string {
	var out []string
	for _, log := range logs {
		out = append(out, fmt.Sprintf("%s %s", log["level"], strings.Trim(log["msg"].(string), "\n")))
	}
	return out
}

func MustFindLogValue(logs []map[string]any, msg string, key string) any {
	for _, log := range logs {
		if log["msg"] == msg {
			val, ok := log[key]
			if !ok {
				panic("no matching key")
			}
			return val
		}
	}
	panic("no matching msg")
}

/*
func maybeRunInSubtest(t *testing.T, runInSubtest bool, name string, fun func(t *testing.T)) {
	if runInSubtest {
		t.Run(name, fun)
	} else {
		fun(t)
	}
}

type SimConfig struct {
	Seeds                []int64
	EnableTracer         bool
	CheckDeterminismRuns int // XXX: ensure this also checks logs
	SimSubtest           bool
	SeedSubtests         bool
	RunSubtests          bool
	DontReportFail       bool // rename?

	CaptureLog       bool
	LogLevelOverride string
	// xxx: time limit here? steps limit?
}

func RunNestedTest(t *testing.T, config SimConfig, test Test) []gosimruntime.RunResult {
	if config.CheckDeterminismRuns > 0 && !config.EnableTracer {
		// XXX: automatically set flag?
		panic("must enable tracer when checking determinism")
	}
	if config.CheckDeterminismRuns < 0 {
		panic("check determinism runs must be non-negative")
	}

	// XXX
	// dontReportFail := config.DontReportFail || s.dontReportFail
	dontReportFail := config.DontReportFail

	var results []gosimruntime.RunResult
	maybeRunInSubtest(t, config.SimSubtest, test.Name, func(t *testing.T) {
		for _, seed := range config.Seeds {
			maybeRunInSubtest(t, config.SeedSubtests, fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
				for run := 0; run < 1+config.CheckDeterminismRuns; run++ {
					maybeRunInSubtest(t, config.RunSubtests, fmt.Sprintf("run-%d", run), func(t *testing.T) {
						result := RunInner(
							test.Test, seed, config.EnableTracer, config.CaptureLog,
							config.LogLevelOverride, dontReportFail, t,
						)

						if run == 0 {
							results = append(results, result)
						} else {
							prevResult := results[len(results)-1]
							if !bytes.Equal(result.Trace, prevResult.Trace) {
								slog.Error("traces differ: non-determinism found")
								t.Fail()
								// XXX debug view?
							}
							if !bytes.Equal(result.LogOutput, prevResult.LogOutput) {
								slog.Error(
									"logs differ: non-determinism found",
									"diff", cmp.Diff(result.LogOutput, prevResult.LogOutput),
								)
								t.Fail()
							}
							results = append(results, result)
						}
					})
				}
			})
		}
	})
	return results
}
*/
