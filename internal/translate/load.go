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

func sourceEnvironment(cfg gosimtool.BuildConfig, application bool) ([]string, error) {
	cmd := buildspec.OutputCommand(cfg.Context, cfg.Toolchain, "", "env")
	if application {
		cmd = buildspec.ApplicationCommand(cfg.Context, cfg.Resolution, "env")
	}
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	overrides := map[string]string{
		"GOOS":             cfg.Identity.SimulationOS,
		"GOARCH":           cfg.Identity.SimulationArch,
		"CGO_ENABLED":      "0",
		"GOPACKAGESDRIVER": "off",
		"GOTOOLCHAIN":      "local",
	}
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

func loadSelectedPackages(
	dir string,
	patterns []string,
	cfg gosimtool.BuildConfig,
	mode packages.LoadMode,
	tests bool,
) ([]*packages.Package, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := validateLoaderWorker(cfg.Toolchain); err != nil {
		return nil, err
	}
	application := dir == cfg.Resolution.Context.Directory
	env, err := sourceEnvironment(cfg, application)
	if err != nil {
		return nil, err
	}
	// Select available Go fallbacks for translated third-party assembly packages.
	// Reviewed native computational packages retain their normal implementations;
	// selected source files participate in package content hashes.
	tags := buildspec.TranslatedSourceTags(cfg.Identity.Race)
	mode |= packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule | packages.NeedImports | packages.NeedEmbedFiles
	flags := []string{"-mod=readonly"}
	if application {
		flags = buildspec.ApplicationFlags(cfg.Resolution)
	}
	flags = append(flags, "-tags="+tags)
	roots, err := packages.Load(
		&packages.Config{
			Context:    cfg.Context,
			Dir:        dir,
			Mode:       mode,
			Tests:      tests,
			Fset:       token.NewFileSet(),
			Env:        env,
			BuildFlags: flags,
		},
		patterns...)
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
