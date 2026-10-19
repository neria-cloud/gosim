package buildcfg

import (
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

type Distribution struct {
	ArchiveDigest, SourceDigest, VerificationOrigin string
}

func Resolve(exact string) (buildspec.Toolchain, error) {
	if err := buildspec.ValidateVersion(exact); err != nil {
		return buildspec.Toolchain{}, err
	}
	if _, err := distributionRecord(exact, runtime.GOOS, runtime.GOARCH); err != nil {
		return buildspec.Toolchain{}, err
	}
	// The launcher may locate/download the exact toolchain, but cannot select a
	// newer version. All subsequent work invokes the resolved executable itself.
	launcher, err := exec.LookPath("go")
	if err != nil {
		return buildspec.Toolchain{}, err
	}
	launcher, err = filepath.Abs(launcher)
	if err != nil {
		return buildspec.Toolchain{}, err
	}
	tc := buildspec.Toolchain{Executable: launcher, Version: exact, GOROOT: runtime.GOROOT(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	cmd := Command(tc, "", "env", "-json", "GOVERSION", "GOROOT", "GOHOSTOS", "GOHOSTARCH")
	// GOROOT must come from the selected distribution, not the launcher's root.
	cmd.Env = withoutEnv(cmd.Env, "GOROOT")
	data, err := cmd.Output()
	if err != nil {
		return tc, fmt.Errorf("resolve %s: %w", exact, err)
	}
	var info struct{ GOVERSION, GOROOT, GOHOSTOS, GOHOSTARCH string }
	if err := json.Unmarshal(data, &info); err != nil {
		return tc, err
	}
	if info.GOVERSION != exact || info.GOHOSTOS != runtime.GOOS || info.GOHOSTARCH != runtime.GOARCH {
		return tc, fmt.Errorf("toolchain resolved to %s %s/%s, requested %s %s/%s", info.GOVERSION, info.GOHOSTOS, info.GOHOSTARCH, exact, runtime.GOOS, runtime.GOARCH)
	}
	tc.GOROOT, err = filepath.EvalSymlinks(info.GOROOT)
	if err != nil {
		return tc, err
	}
	tc.Executable = filepath.Join(tc.GOROOT, "bin", "go")
	if err := validateExecutable(tc); err != nil {
		return tc, err
	}
	return tc, nil
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

func Fingerprint(tc buildspec.Toolchain) (Distribution, error) {
	record, err := distributionRecord(tc.Version, tc.HostOS, tc.HostArch)
	if err != nil {
		return Distribution{}, err
	}
	if tc.Executable != filepath.Join(tc.GOROOT, "bin", "go") {
		return Distribution{}, fmt.Errorf("executable is outside selected distribution")
	}
	cache := os.Getenv("GOSIMCACHE")
	if cache == "" {
		cache, err = os.UserCacheDir()
		if err != nil {
			return Distribution{}, err
		}
	}
	archive, err := fetchArchive(record, filepath.Join(cache, "gosim", "distributions"))
	if err != nil {
		return Distribution{}, err
	}
	defer archive.Close()
	return verifyDistribution(tc.GOROOT, archive, record)
}
