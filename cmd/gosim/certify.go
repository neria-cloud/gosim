package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildrun"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/tests/features"
)

type (
	MatrixOptions       = buildspec.MatrixOptions
	CertificationReport = buildspec.CertificationReport
	SuiteResult         = buildspec.SuiteResult
)

func runCertificationCommand(command string, args []string) {
	f := flag.NewFlagSet("internal-candidate "+command, flag.ExitOnError)
	flags := addBuildFlags(f)
	options := MatrixOptions{}
	f.StringVar(&options.OutputDir, "output", ".gosim/certification", "evidence output directory")
	f.StringVar(&options.ExecutionMode, "execution-mode", "native", "native|container-native|container-emulated")
	f.StringVar(&options.ContainerImageDigest, "image-digest", "", "resolved container image sha256 digest")
	f.Parse(args)
	if f.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected certification arguments")
		os.Exit(2)
	}
	var err error
	if command == "verify" {
		err = verifyStep(*flags, options.OutputDir)
	} else {
		_, err = certify(*flags, options)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// certify records one declared pending tuple. Reports stay candidate and never
// edit reviewed profile metadata. Package applicability comes from one registry.
func certify(flags BuildFlags, options MatrixOptions) (report CertificationReport, err error) {
	resolved, err := resolveBuild("internal-candidate", flags)
	if err != nil {
		return report, err
	}
	report = CertificationReport{Schema: 1, Tuple: buildspec.TupleFor(resolved.Identity), Identity: resolved.Identity, ExecutionMode: options.ExecutionMode, WorkerArch: runtime.GOARCH, ContainerImageDigest: options.ContainerImageDigest}
	if err := buildspec.ValidateExecution(resolved.Identity, runtime.GOARCH, options.ExecutionMode, options.ContainerImageDigest); err != nil {
		return report, err
	}
	cfg, err := gosimtool.NewBuildConfig(resolved.Selection, resolved.Toolchain)
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
	report.Suites, err = runFullSuite(cfg, dir)
	if err != nil {
		return report, err
	}
	report.ResultDigest, _ = buildspec.ReportDigest(report)
	return report, buildspec.ValidateReport(report)
}

// verifyStep and certify share the full suite runner; neither recurses through
// the shell entry point or claims compatibility promotion.
func verifyStep(flags BuildFlags, output string) error {
	if err := exec.Command("./.ci/gofmt", "check").Run(); err != nil {
		return err
	}
	for _, race := range []bool{false, true} {
		flags.Options.Race = race
		resolved, err := resolveBuild("internal-candidate", flags)
		if err != nil {
			return err
		}
		if len(resolved.Identity.Features) != 3 {
			return fmt.Errorf("verify requires rt,fs,net")
		}
		cfg, err := gosimtool.NewBuildConfig(resolved.Selection, resolved.Toolchain)
		if err != nil {
			return err
		}
		dir := filepath.Join(output, cfg.Digest)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if _, err := runFullSuite(cfg, dir); err != nil {
			return err
		}
	}
	return nil
}

func runFullSuite(cfg gosimtool.BuildConfig, dir string) (results []SuiteResult, err error) {
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
		buildErr = fmt.Errorf("missing applicable package artifacts: got %d want %d", len(built.Artifacts), len(packages))
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
	var execution bytes.Buffer
	for _, artifact := range built.Artifacts {
		if strings.Contains(artifact.Manifest.PackageDigests[0].Path, "/internal_/tests/race/testdata") {
			continue
		} // intentionally racy oracle is checked by native harness
		cmd := exec.Command(filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path), "-test.v")
		output, runErr := cmd.CombinedOutput()
		execution.Write(output)
		if runErr != nil {
			err = errors.Join(err, runErr)
		}
	}
	if writeErr := os.WriteFile(filepath.Join(dir, "execute.log"), execution.Bytes(), 0o644); writeErr != nil {
		return results, writeErr
	}
	results = append(results, SuiteResult{Name: "execute", Passed: err == nil, LogDigest: buildspec.Hash(execution.Bytes())})
	if err != nil {
		return results, err
	}
	args := []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "-timeout=30m", "-count=1"}
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
	config := buildspec.Config{Schema: 1, Toolchain: cfg.Identity.Toolchain, Features: cfg.Identity.Features, SimulationOS: cfg.Identity.SimulationOS}
	configData, _ := json.Marshal(config)
	configPath, err := filepath.Abs(filepath.Join(dir, "gosim.json"))
	if err != nil {
		return results, err
	}
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		return results, err
	}
	worker, err := candidateWrapper()
	if err != nil {
		return results, err
	}
	cmd.Env = append(cmd.Env, "GOSIM_CONFIG="+configPath, "GOSIMTOOL="+worker)
	result, err := runLogged(cmd, dir, "native")
	results = append(results, result)
	if err != nil || full {
		return results, err
	}
	units, err := features.NativePackages(cfg.Identity.Features)
	if err != nil {
		return results, err
	}
	args = []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "-timeout=30m", "-count=1"}
	if cfg.Identity.Race {
		args = append(args, "-race")
	}
	args = append(args, units...)
	unitCommand := buildcfg.Command(cfg.Toolchain, "", args...)
	unitCommand.Env = cmd.Env
	result, err = runLogged(unitCommand, dir, "native-units")
	results = append(results, result)
	if err != nil {
		return results, err
	}
	// Full native testing includes all candidate workers. Reduced certification
	// also executes its isolated negative, replay and generated-graph checks.
	args = []string{"test", "-ldflags=-checklinkname=0", "-tags=linkname", "-timeout=25m", "./internal/candidate", "-run=^TestCandidateWorker$/^" + strings.Join(cfg.Identity.Features, ",") + "$", "-count=1", "-v"}
	if cfg.Identity.Race {
		args = append(args, "-race")
	}
	cmd = buildcfg.Command(cfg.Toolchain, "", args...)
	cmd.Env = append(cmd.Env, "GOSIM_CANDIDATE_WORKER=1")
	result, err = runLogged(cmd, dir, "boundaries")
	results = append(results, result)
	return results, err
}

func runLogged(cmd *exec.Cmd, dir, name string) (SuiteResult, error) {
	output, err := cmd.CombinedOutput()
	// Preserve stdout/stderr even on failure; a failed report remains incomplete.
	if writeErr := os.WriteFile(filepath.Join(dir, name+".log"), output, 0o644); writeErr != nil {
		return SuiteResult{}, errors.Join(err, writeErr)
	}
	fmt.Fprintf(os.Stdout, "%s: passed=%t (%s)\n", name, err == nil, filepath.Join(dir, name+".log"))
	return SuiteResult{Name: name, Passed: err == nil, LogDigest: buildspec.Hash(output)}, err
}

func candidateWrapper() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	root, err := gosimtool.FindGoModDir()
	if err != nil {
		return "", err
	}
	destination := filepath.Join(root, ".gosim", "ci", runtime.Version(), "gosim-candidate")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	data := []byte("#!/bin/sh\nexec " + quote(executable) + " internal-candidate \"$@\"\n")
	if err := os.WriteFile(destination, data, 0o755); err != nil {
		return "", err
	}
	return destination, nil
}
