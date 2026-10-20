package gosimcli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/workerboot"
)

type BuildFlags struct {
	ConfigPath string
	Overrides  buildcfg.Overrides
	Options    buildspec.BuildOptions
}
type resolutionKey struct{}

type ResolvedBuild struct {
	Resolution buildspec.Resolution
	Config     buildspec.Config
	Toolchain  buildspec.Toolchain
	Identity   buildspec.Identity
	Selection  buildspec.Selection
}

func addBuildFlags(f *flag.FlagSet) *BuildFlags {
	b := new(BuildFlags)
	f.StringVar(&b.ConfigPath, "config", os.Getenv("GOSIM_CONFIG"), "build configuration path")
	f.Func("go", "exact Go toolchain", func(s string) error { b.Overrides.Toolchain = &s; return nil })
	f.Func(
		"features",
		"complete comma-separated feature set",
		func(s string) error { v := strings.Split(s, ","); b.Overrides.Features = &v; return nil },
	)
	f.BoolVar(&b.Options.Race, "race", false, "enable race instrumentation")
	return b
}

func resolveBuild(command string, flags BuildFlags) (ResolvedBuild, error) {
	return resolveBuildForContext(context.Background(), command, flags)
}

func resolveBuildForContext(ctx context.Context, command string, flags BuildFlags) (ResolvedBuild, error) {
	var r ResolvedBuild
	cwd, err := os.Getwd()
	if err != nil {
		return r, err
	}
	path, err := workerboot.ConfigPath(cwd, flags.ConfigPath)
	if err != nil {
		return r, err
	}
	request, err := buildcfg.Load(path, flags.Overrides)
	if err != nil {
		return r, err
	}
	// packages.Load resolves go from the worker PATH before applying Config.Env.
	if request.Legacy {
		fmt.Fprintln(
			os.Stderr,
			"gosim: legacy schema-1 toolchain override; migrate gosim.json to schema 2 for automatic Go selection",
		)
	}
	invocation, handed := ctx.Value(resolutionKey{}).(workerboot.Invocation)
	resolution := invocation.Resolution
	if !handed {
		resolution, err = buildcfg.ResolveRunningProcess(ctx, "")
	}
	if err == nil && request.ExplicitToolchain != "" && request.ExplicitToolchain != resolution.Toolchain.Version {
		err = fmt.Errorf("worker compiler differs from requested override; use the gosim launcher")
	}
	tc := resolution.Toolchain
	config := request.Config
	if err != nil {
		return r, err
	}
	if err := buildcfg.ValidateWorker(tc); err != nil {
		return r, err
	}
	if err := os.Setenv("PATH", filepath.Dir(tc.Executable)+string(filepath.ListSeparator)+os.Getenv("PATH")); err != nil {
		return r, err
	}
	profile := selected.Profile()
	inputs := invocation.Fingerprints
	if !handed {
		inputs, err = buildcfg.Fingerprint(tc)
	}
	if err != nil {
		return r, err
	}
	contract, err := spec.CheckContracts(ctx, resolution, profile, flags.Options, inputs, "")
	if err != nil {
		return r, err
	}
	id, err := buildcfg.Identity(ctx, config, resolution, profile, flags.Options, inputs, contract)
	if err != nil {
		return r, err
	}

	selection, err := profile.Select(id)
	if err != nil {
		return r, err
	}
	return ResolvedBuild{Config: config, Resolution: resolution, Toolchain: tc, Identity: id, Selection: selection}, nil
}
