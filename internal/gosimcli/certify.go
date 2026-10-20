package gosimcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildrun"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/tests/features"
	"github.com/neria-cloud/gosim/internal/workerboot"
)

type (
	MatrixOptions       = buildspec.MatrixOptions
	CertificationReport = buildspec.CertificationReport
	SuiteResult         = buildspec.SuiteResult
)

type suiteOptions struct {
	concurrency int
	parallel    int
	packages    int
	json        bool
	large       bool
}

func runCertificationCommand(
	ctx context.Context,
	command string,
	args []string,
	streams workerboot.Streams,
) (int, error) {
	f := flag.NewFlagSet("internal-candidate "+command, flag.ContinueOnError)
	f.SetOutput(streams.Err)
	flags := addBuildFlags(f)
	options := MatrixOptions{}
	suite := suiteOptions{}
	ordinary := false
	if command == "verify" {
		f.BoolVar(&ordinary, "ordinary", false, "run ordinary tests (default); incompatible with -race")
	}
	f.IntVar(&suite.concurrency, "concurrency", min(4, runtime.GOMAXPROCS(0)), "maximum concurrent test workers")
	f.IntVar(&suite.parallel, "test-parallel", 0, "parallel native tests per package (zero divides the worker budget)")
	f.IntVar(&suite.packages, "test-packages", 0, "parallel native packages (zero divides the worker budget)")
	f.BoolVar(&suite.json, "json", false, "stream Go JSON events for persistent test reports")
	f.BoolVar(&suite.large, "large", false, "include selected-tuple make test-large coverage for certification")
	defaultOutput := ".gosim/certification"
	if command == "verify" {
		defaultOutput = ".gosim/verification"
	}
	f.StringVar(&options.OutputDir, "output", defaultOutput, "suite output directory")
	f.StringVar(&options.ExecutionMode, "execution-mode", "native", "native|container-native|container-emulated")
	f.StringVar(&options.ContainerImageDigest, "image-digest", "", "resolved container image sha256 digest")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}
	if f.NArg() != 0 || suite.concurrency < 1 || suite.parallel < 0 || suite.packages < 0 ||
		(ordinary && flags.Options.Race) {
		return 2, errors.New("require no positional arguments, positive -concurrency, and one test mode")
	}
	var err error
	if command == "verify" {
		err = verifyStepContext(ctx, *flags, options.OutputDir, suite)
	} else {
		_, err = certifyContext(ctx, *flags, options, suite)
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}

// certify is a manually invoked exact-build report. It never edits profile
// metadata or controls ordinary use. Package applicability comes from one registry.
func certify(flags BuildFlags, options MatrixOptions, suite suiteOptions) (CertificationReport, error) {
	return certifyContext(context.Background(), flags, options, suite)
}

func certifyContext(
	ctx context.Context,
	flags BuildFlags,
	options MatrixOptions,
	suite suiteOptions,
) (report CertificationReport, err error) {
	if err := validateCertificationSelection(runtime.GOARCH, flags.Options.Race); err != nil {
		return report, err
	}
	resolved, err := resolveBuildForContext(ctx, "internal-candidate", flags)
	if err != nil {
		return report, err
	}
	if !suite.large {
		return report, errors.New(
			"manual certification requires explicit -large coverage; use verify for ordinary tests",
		)
	}
	report = CertificationReport{
		Schema:               2,
		Tuple:                buildspec.TupleFor(resolved.Identity),
		Identity:             resolved.Identity,
		ExecutionMode:        options.ExecutionMode,
		WorkerArch:           runtime.GOARCH,
		ContainerImageDigest: options.ContainerImageDigest,
	}
	report.FrameworkDigest, err = buildspec.SourceDigest(resolved.Identity)
	if err != nil {
		return report, err
	}
	if err := buildspec.ValidateExecution(resolved.Identity, runtime.GOARCH, options.ExecutionMode, options.ContainerImageDigest); err != nil {
		return report, err
	}
	cfg, err := gosimtool.NewBuildConfig(ctx, resolved.Selection, resolved.Resolution)
	if err != nil {
		return report, err
	}
	dir, err := filepath.Abs(filepath.Join(options.OutputDir, cfg.Digest))
	if err != nil {
		return report, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return report, err
	}
	dir, err = os.MkdirTemp(dir, "run-")
	if err != nil {
		return report, err
	}
	defer func() {
		fmt.Fprintln(os.Stdout, "report:", filepath.Join(dir, "report.json"))
		report.ResultDigest, _ = buildspec.ReportDigest(report)
		data, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr == nil {
			marshalErr = os.WriteFile(filepath.Join(dir, "report.json"), append(data, '\n'), 0o644)
		}
		err = errors.Join(err, marshalErr)
	}()
	report.Suites, err = runFullSuite(cfg, dir, suite)
	if err != nil {
		return report, err
	}
	report.ResultDigest, _ = buildspec.ReportDigest(report)
	return report, buildspec.ValidateReport(report)
}

// Only ARM64 builds without race instrumentation may produce manual certification.
func validateCertificationSelection(hostArch string, race bool) error {
	if hostArch != "arm64" || race {
		return errors.New("AMD64 and race certification are disabled; use ARM64 without race")
	}
	return nil
}

// verifyStep runs only the requested local test mode and writes suite logs.
// It does not create a certification report or implicitly add race/large suites.
func verifyStep(flags BuildFlags, output string, suite suiteOptions) error {
	return verifyStepContext(context.Background(), flags, output, suite)
}

func verifyStepContext(ctx context.Context, flags BuildFlags, output string, suite suiteOptions) error {
	if suite.large {
		return errors.New("large suites are separate; use make test-large explicitly")
	}
	resolved, err := resolveBuildForContext(ctx, "internal-candidate", flags)
	if err != nil {
		return err
	}
	if len(resolved.Identity.Features) != 3 {
		return errors.New("verify requires rt,fs,net")
	}
	cfg, err := gosimtool.NewBuildConfig(ctx, resolved.Selection, resolved.Resolution)
	if err != nil {
		return err
	}
	dir := filepath.Join(output, cfg.Digest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_, err = runFullSuite(cfg, dir, suite)
	return err
}

func runFullSuite(cfg gosimtool.BuildConfig, dir string, suite suiteOptions) (results []SuiteResult, err error) {
	if err := prepareSelftest(cfg); err != nil {
		return results, err
	}
	packages, err := features.Packages(cfg.Identity.Features)
	if err != nil {
		return results, err
	}
	full := len(cfg.Identity.Features) == 3
	if full && cfg.Identity.Race {
		packages = append(packages, "./internal/tests/race/testdata")
	}
	built, buildErr := buildrun.BuildResolved(cfg, packages)
	if buildErr == nil && len(built.Artifacts) != len(packages) {
		buildErr = fmt.Errorf(
			"missing applicable package artifacts: got %d want %d",
			len(built.Artifacts),
			len(packages),
		)
	}
	data, marshalErr := json.Marshal(built.Artifacts)
	buildErr = errors.Join(buildErr, marshalErr)
	if buildErr != nil {
		data = append(data, []byte("\n"+buildErr.Error())...)
	}
	if writeErr := os.WriteFile(filepath.Join(dir, "build.log"), data, 0o644); writeErr != nil {
		return results, writeErr
	}
	results = append(results, SuiteResult{Name: "build", Passed: buildErr == nil, LogDigest: buildspec.Hash(data)})
	if buildErr != nil {
		return results, buildErr
	}
	var commands []*exec.Cmd
	var converters []*exec.Cmd
	for _, artifact := range built.Artifacts {
		if strings.Contains(artifact.Manifest.PackageDigests[0].Path, "/internal_/tests/race/testdata") {
			continue
		} // intentionally racy oracle is checked by native harness
		binary := filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path)
		cmd := exec.Command(binary, "-test.v")
		var converter *exec.Cmd
		if suite.json {
			pkg := strings.TrimPrefix(artifact.Manifest.PackageDigests[0].Path, "translated/"+gosimtool.Module+"/")
			pkg = strings.ReplaceAll(pkg, "internal_", "internal")
			converter = buildcfg.Command(
				cfg.Toolchain,
				"",
				"tool",
				"test2json",
				"-t",
				"-p",
				gosimtool.Module+"/simulation/"+pkg,
			)
		}
		commands = append(commands, cmd)
		converters = append(converters, converter)
	}
	var stream io.Writer
	if suite.json {
		stream = os.Stdout
	}
	var outputLock sync.Mutex
	execution, err := runTestJobs(len(commands), suite.concurrency, func(i int) ([]byte, error) {
		if converters[i] != nil {
			return captureSimulationCommand(cfg.Context, commands[i], converters[i], &outputLock, stream)
		}
		return captureCommand(cfg.Context, commands[i], &outputLock, stream)
	})
	if writeErr := os.WriteFile(filepath.Join(dir, "execute.log"), execution, 0o644); writeErr != nil {
		return results, writeErr
	}
	results = append(results, SuiteResult{Name: "execute", Passed: err == nil, LogDigest: buildspec.Hash(execution)})
	if err != nil {
		return results, err
	}
	args := []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "-timeout=30m", "-count=1"}
	args = append(args, suite.nativeFlags()...)
	if full {
		args = append(args, "./...")
	} else {
		args = append(args, packages...)
	}
	if cfg.Identity.Race {
		args = append(args, "-race")
	}
	cmd := buildcfg.Command(cfg.Toolchain, "", args...)
	// This is a native host test binary. Let its Go tool use the validated
	// native default instead of exporting an architecture override into tests
	// that intentionally inspect both Linux source ABIs. Their own commands
	// select an explicit architecture and retain environment validation.
	for i, entry := range cmd.Env {
		if strings.HasPrefix(entry, "GOARCH=") {
			cmd.Env[i] = "GOARCH="
		}
	}
	config := buildspec.Config{Schema: 2, Features: cfg.Identity.Features, SimulationOS: cfg.Identity.SimulationOS}
	configData, _ := json.Marshal(config)
	configPath, err := filepath.Abs(filepath.Join(dir, "gosim.json"))
	if err != nil {
		return results, err
	}
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		return results, err
	}
	cmd.Env = append(cmd.Env, "GOSIM_CONFIG="+configPath, "GOSIMTOOL=", "GOSIM_TEST_LARGE=0")
	result, err := runLogged(cfg.Context, cmd, dir, "native", stream)
	results = append(results, result)
	if err != nil || full {
		if err == nil && suite.large {
			result, err = runLargeSuite(cfg, dir, suite)
			results = append(results, result)
		}
		return results, err
	}
	units, err := features.NativePackages(cfg.Identity.Features)
	if err != nil {
		return results, err
	}
	args = []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "-timeout=30m", "-count=1"}
	args = append(args, suite.nativeFlags()...)
	if cfg.Identity.Race {
		args = append(args, "-race")
	}
	args = append(args, units...)
	unitCommand := buildcfg.Command(cfg.Toolchain, "", args...)
	unitCommand.Env = cmd.Env
	result, err = runLogged(cfg.Context, unitCommand, dir, "native-units", stream)
	results = append(results, result)
	if err != nil {
		return results, err
	}
	if suite.large {
		result, err = runLargeSuite(cfg, dir, suite)
		results = append(results, result)
	}
	return results, err
}

func runLargeSuite(cfg gosimtool.BuildConfig, dir string, suite suiteOptions) (SuiteResult, error) {
	parallel, packages := max(1, suite.concurrency/2), 2
	if suite.concurrency == 1 {
		packages = 1
	}
	if suite.parallel > 0 {
		parallel = suite.parallel
	}
	if suite.packages > 0 {
		packages = suite.packages
	}
	race := "0"
	if cfg.Identity.Race {
		race = "1"
	}
	cmd := exec.Command("make", "test-large", "LARGE_TUPLE="+strings.Join(cfg.Identity.Features, ","),
		"LARGE_GO="+cfg.Toolchain.Executable, "LARGE_RACE="+race,
		"UNIT_PARALLEL="+strconv.Itoa(parallel), "UNIT_PACKAGES="+strconv.Itoa(packages),
		"GOSIM_TOOLCHAIN="+cfg.Identity.Toolchain, "TEST_REPORTS_DIR="+filepath.Join(dir, "test-reports"))
	cmd.Env = buildcfg.Command(cfg.Toolchain, "").Env
	return runLogged(cfg.Context, cmd, dir, "large")
}

// Divide one budget between package processes and parallel tests in each
// process, rather than allowing both levels to multiply the worker count.
func nativeTestParallelFlags(concurrency int) []string {
	packages := max(1, concurrency/2)
	return []string{"-p=" + strconv.Itoa(packages), "-parallel=" + strconv.Itoa(max(1, concurrency/packages))}
}

// Simulation engines cannot share a process. Run independent binaries with a
// bounded pool and merge their logs in input order, regardless of finish order.
func runTestCommands(ctx context.Context, commands []*exec.Cmd, concurrency int, stream ...io.Writer) ([]byte, error) {
	var outputLock sync.Mutex
	return runTestJobs(len(commands), concurrency, func(i int) ([]byte, error) {
		return captureCommand(ctx, commands[i], &outputLock, stream...)
	})
}

func runTestJobs(count, concurrency int, run func(int) ([]byte, error)) ([]byte, error) {
	if concurrency < 1 {
		return nil, errors.New("test concurrency must be positive")
	}
	outputs := make([][]byte, count)
	errs := make([]error, count)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(concurrency, count) {
		wg.Go(func() {
			for i := range jobs {
				outputs[i], errs[i] = run(i)
				if errs[i] != nil {
					errs[i] = fmt.Errorf("test worker %d: %w", i, errs[i])
				}
			}
		})
	}
	for i := range count {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return bytes.Join(outputs, nil), errors.Join(errs...)
}

func runLogged(ctx context.Context, cmd *exec.Cmd, dir, name string, stream ...io.Writer) (SuiteResult, error) {
	output, err := captureCommand(ctx, cmd, new(sync.Mutex), stream...)
	// Preserve stdout/stderr even on failure; a failed report remains incomplete.
	if writeErr := os.WriteFile(filepath.Join(dir, name+".log"), output, 0o644); writeErr != nil {
		return SuiteResult{}, errors.Join(err, writeErr)
	}
	fmt.Fprintf(os.Stdout, "%s: passed=%t (%s)\n", name, err == nil, filepath.Join(dir, name+".log"))
	return SuiteResult{Name: name, Passed: err == nil, LogDigest: buildspec.Hash(output)}, err
}

// Each command buffers partial lines before taking the shared output lock.
// Parallel test2json processes must never interleave bytes in a JSON event.
type eventWriter struct {
	pending []byte
	mu      *sync.Mutex
	out     io.Writer
}

func (w *eventWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		end := bytes.IndexByte(w.pending, '\n')
		if end < 0 {
			break
		}
		w.mu.Lock()
		_, err := w.out.Write(w.pending[:end+1])
		w.mu.Unlock()
		if err != nil {
			return 0, err
		}
		w.pending = w.pending[end+1:]
	}
	return len(p), nil
}

func captureCommand(ctx context.Context, cmd *exec.Cmd, mu *sync.Mutex, stream ...io.Writer) ([]byte, error) {
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	var events *eventWriter
	if len(stream) > 0 && stream[0] != nil {
		events = &eventWriter{mu: mu, out: stream[0]}
		cmd.Stdout = io.MultiWriter(&output, events)
		cmd.Stderr = cmd.Stdout
	}
	err := buildspec.RunProcess(ctx, cmd)
	if events != nil && len(events.pending) > 0 {
		mu.Lock()
		_, writeErr := events.out.Write(events.pending)
		mu.Unlock()
		err = errors.Join(err, writeErr)
	}
	return output.Bytes(), err
}

func (s suiteOptions) nativeFlags() []string {
	flags := nativeTestParallelFlags(s.concurrency)
	if s.packages > 0 {
		flags[0] = "-p=" + strconv.Itoa(s.packages)
	}
	if s.parallel > 0 {
		flags[1] = "-parallel=" + strconv.Itoa(s.parallel)
	}
	if s.json {
		flags = append(flags, "-json")
	}
	return flags
}
