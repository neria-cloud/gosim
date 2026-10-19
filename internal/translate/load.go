package translate

import (
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

// go/packages invokes "go" through the worker's PATH, not Config.Env's PATH.
// Pinned child commands install this PATH before starting the worker. Reject an
// unbound caller rather than changing process-wide environment during loading.
func validateLoaderWorker(tc buildspec.Toolchain) error {
	if err := buildcfg.ValidateWorker(tc); err != nil {
		return err
	}
	executable, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	expected, err := filepath.EvalSymlinks(tc.Executable)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("package-loader worker PATH selects %s, require pinned executable %s", actual, expected)
	}
	return nil
}

func sourceEnvironment(cfg gosimtool.BuildConfig) ([]string, error) {
	cmd := buildcfg.Command(cfg.Toolchain, "", "env")
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	overrides := map[string]string{"GOOS": cfg.Identity.SimulationOS, "GOARCH": cfg.Identity.SimulationArch, "CGO_ENABLED": "0", "GOPACKAGESDRIVER": "off", "GOTOOLCHAIN": "local"}
	var env []string
	for _, entry := range cmd.Env {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := overrides[key]; !replace {
			env = append(env, entry)
		}
	}
	for _, key := range []string{"GOOS", "GOARCH", "CGO_ENABLED", "GOPACKAGESDRIVER", "GOTOOLCHAIN"} {
		env = append(env, key+"="+overrides[key])
	}
	return env, nil
}

func loadSelectedPackages(dir string, patterns []string, cfg gosimtool.BuildConfig, mode packages.LoadMode, tests bool) ([]*packages.Package, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := validateLoaderWorker(cfg.Toolchain); err != nil {
		return nil, err
	}
	env, err := sourceEnvironment(cfg)
	if err != nil {
		return nil, err
	}
	// math/big supplies complete Go arithmetic for both reviewed families. This
	// owned source tag avoids copying private assembly into translated packages;
	// the actually selected files participate in package content hashes.
	tags := "linkname,sim,math_big_pure_go"
	// Application source remains cgo-free. This is an owned source-selection tag,
	// not a user build-flag override or a request to compile a cgo race executable.
	if cfg.Identity.Race {
		tags += ",race"
	}
	mode |= packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule | packages.NeedImports | packages.NeedEmbedFiles
	roots, err := packages.Load(&packages.Config{Dir: dir, Mode: mode, Tests: tests, Fset: token.NewFileSet(), Env: env, BuildFlags: []string{"-mod=readonly", "-tags=" + tags}}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load selected sources: %w", err)
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no packages matched %v", patterns)
	}
	for _, pkg := range roots {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("load %s: %s", pkg.ID, pkg.Errors[0])
		}
	}
	return roots, nil
}

func contentDigests(files []string) (map[string]string, error) {
	result := make(map[string]string, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("source dependency %s: %w", path, err)
		}
		result[path] = buildspec.Hash(data)
	}
	return result, nil
}
