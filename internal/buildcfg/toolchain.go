package buildcfg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// Discover lets the caller's Go command perform its normal module/workspace and
// GOTOOLCHAIN resolution before freezing a concrete executable for this operation.
func Discover(ctx context.Context, directory, explicitVersion string) (buildspec.Resolution, error) {
	if explicitVersion != "" {
		if err := buildspec.ValidateVersion(explicitVersion); err != nil {
			return buildspec.Resolution{}, err
		}
	}
	launcher, err := exec.LookPath("go")
	if err != nil {
		return buildspec.Resolution{}, err
	}
	env := os.Environ()
	if explicitVersion != "" {
		env = append(withoutEnv(withoutEnv(env, "GOTOOLCHAIN"), "GOROOT"), "GOTOOLCHAIN="+explicitVersion)
	}
	r, err := discoverWith(ctx, launcher, directory, env)
	if err == nil && explicitVersion != "" && r.Toolchain.Version != explicitVersion {
		return r, fmt.Errorf("Go resolved %s, explicitly requested %s", r.Toolchain.Version, explicitVersion)
	}
	return r, err
}

func discoverWith(ctx context.Context, launcher, directory string, env []string) (buildspec.Resolution, error) {
	var result buildspec.Resolution
	directory, err := filepath.Abs(directory)
	if err != nil {
		return result, err
	}
	cmd := exec.CommandContext(ctx, launcher, "env", "-json", "GOVERSION", "GOROOT", "GOHOSTOS", "GOHOSTARCH", "GOMOD", "GOWORK")
	cmd.Dir, cmd.Env = directory, env
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		return result, fmt.Errorf("resolve Go in %s: %w\n%s", directory, err, diagnostics.Bytes())
	}
	var info struct{ GOVERSION, GOROOT, GOHOSTOS, GOHOSTARCH, GOMOD, GOWORK string }
	if err := json.Unmarshal(data, &info); err != nil {
		return result, fmt.Errorf("decode Go resolution: %w", err)
	}
	if err := buildspec.ValidateVersion(info.GOVERSION); err != nil {
		return result, err
	}
	if info.GOHOSTOS != runtime.GOOS || info.GOHOSTARCH != runtime.GOARCH {
		return result, fmt.Errorf("resolved Go host %s/%s differs from local %s/%s", info.GOHOSTOS, info.GOHOSTARCH, runtime.GOOS, runtime.GOARCH)
	}
	if err := buildspec.ValidatePlatform(info.GOHOSTOS, info.GOHOSTARCH, "linux", info.GOHOSTARCH); err != nil {
		return result, err
	}
	root, err := filepath.EvalSymlinks(info.GOROOT)
	if err != nil {
		return result, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return result, err
	}
	tc := buildspec.Toolchain{Executable: filepath.Join(root, "bin", "go"), Version: info.GOVERSION, GOROOT: root, HostOS: info.GOHOSTOS, HostArch: info.GOHOSTARCH}
	// Check the resolved executable directly. Do not re-enter the launcher's module.
	version := exec.CommandContext(ctx, tc.Executable, "version")
	version.Env = append(withoutEnv(withoutEnv(env, "GOTOOLCHAIN"), "GOROOT"), "GOTOOLCHAIN=local", "GOROOT="+root)
	actual, err := version.Output()
	want := "go version " + tc.Version + " " + tc.HostOS + "/" + tc.HostArch
	if err != nil || strings.TrimSpace(string(actual)) != want {
		return result, fmt.Errorf("selected Go executable identity mismatch: got %q, expected %q: %v", actual, want, err)
	}
	if info.GOWORK == "" {
		info.GOWORK = "off"
	}
	result = buildspec.Resolution{Schema: 1, Toolchain: tc, Context: buildspec.ModuleContext{Directory: directory, GoMod: info.GOMOD, GoWork: info.GOWORK}}
	return result, nil
}

func Resolve(exact string) (buildspec.Toolchain, error) {
	r, err := Discover(context.Background(), "", exact)
	return r.Toolchain, err
}

// ResolveRunningProcess respects the compiler Go already chose for a native test.
func ResolveRunningProcess(ctx context.Context, directory string) (buildspec.Resolution, error) {
	root := runtime.GOROOT()
	env := append(withoutEnv(withoutEnv(os.Environ(), "GOTOOLCHAIN"), "GOROOT"), "GOTOOLCHAIN=local", "GOROOT="+root)
	r, err := discoverWith(ctx, filepath.Join(root, "bin", "go"), directory, env)
	if err != nil {
		return r, fmt.Errorf("resolve running Go test context; re-run go test if module requirements changed: %w", err)
	}
	if r.Toolchain.Version != runtime.Version() {
		return r, fmt.Errorf("running test compiler %s differs from installed %s; re-run go test", runtime.Version(), r.Toolchain.Version)
	}
	return r, nil
}

func withoutEnv(env []string, key string) []string {
	var result []string
	for _, entry := range env {
		if !strings.HasPrefix(entry, key+"=") {
			result = append(result, entry)
		}
	}
	return result
}

func validateExecutable(tc buildspec.Toolchain) error {
	if tc.Executable != filepath.Join(tc.GOROOT, "bin", "go") {
		return fmt.Errorf("selected executable must be GOROOT/bin/go")
	}
	output, err := Command(tc, "", "version").Output()
	if err != nil {
		return fmt.Errorf("inspect selected go executable: %w", err)
	}
	want := "go version " + tc.Version + " " + tc.HostOS + "/" + tc.HostArch
	if strings.TrimSpace(string(output)) != want {
		return fmt.Errorf("selected executable identity mismatch: got %q, want %q", strings.TrimSpace(string(output)), want)
	}
	return nil
}

func ValidateWorker(tc buildspec.Toolchain) error {
	if runtime.Compiler != "gc" || runtime.Version() != tc.Version || runtime.GOOS != tc.HostOS || runtime.GOARCH != tc.HostArch {
		// TODO(not planned): automatic cross-toolchain worker bootstrap.
		return fmt.Errorf("worker is %s %s/%s; rebuild using GOTOOLCHAIN=%s go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool github.com/neria-cloud/gosim/cmd/gosim", runtime.Version(), runtime.GOOS, runtime.GOARCH, tc.Version)
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if (setting.Key == "GOEXPERIMENT" && setting.Value != "") ||
				(setting.Key == "-compiler" && setting.Value != "gc") ||
				(setting.Key == "-tags" && setting.Value != "" && setting.Value != "linkname") {
				return fmt.Errorf("worker built with unapproved %s=%q", setting.Key, setting.Value)
			}
		}
	}
	root, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		return err
	}
	if root != tc.GOROOT {
		return fmt.Errorf("worker GOROOT %q differs from selected %q", root, tc.GOROOT)
	}
	return validateExecutable(tc)
}

func Command(tc buildspec.Toolchain, dir string, args ...string) *exec.Cmd {
	return buildspec.GoCommand(tc, dir, args...)
}

func Fingerprint(tc buildspec.Toolchain) (buildspec.Fingerprints, error) {
	if err := buildspec.ValidateVersion(tc.Version); err != nil {
		return buildspec.Fingerprints{}, err
	}
	if tc.Executable != filepath.Join(tc.GOROOT, "bin", "go") {
		return buildspec.Fingerprints{}, fmt.Errorf("executable is outside selected distribution")
	}
	return fingerprintInstalled(tc.GOROOT)
}
