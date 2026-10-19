package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/candidate"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

type BuildFlags struct {
	ConfigPath string
	Overrides  buildcfg.Overrides
	Options    buildspec.BuildOptions
}
type ResolvedBuild struct {
	Config    buildspec.Config
	Toolchain buildspec.Toolchain
	Identity  buildspec.Identity
	Selection buildspec.Selection
}

func addBuildFlags(f *flag.FlagSet) *BuildFlags {
	b := new(BuildFlags)
	f.StringVar(&b.ConfigPath, "config", os.Getenv("GOSIM_CONFIG"), "build configuration path")
	f.Func("go", "exact Go toolchain", func(s string) error { b.Overrides.Toolchain = &s; return nil })
	f.Func("features", "complete comma-separated feature set", func(s string) error { v := strings.Split(s, ","); b.Overrides.Features = &v; return nil })
	f.BoolVar(&b.Options.Race, "race", false, "enable race instrumentation")
	return b
}

func resolveBuild(command string, flags BuildFlags) (ResolvedBuild, error) {
	var r ResolvedBuild
	path := flags.ConfigPath
	if path == "" {
		root, err := gosimtool.FindGoModDir()
		if err != nil {
			return r, err
		}
		path = filepath.Join(root, "gosim.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = ""
		}
	}
	config, err := buildcfg.Load(path, flags.Overrides)
	if err != nil {
		return r, err
	}
	// packages.Load resolves go from the worker PATH before applying Config.Env.
	tc, err := buildcfg.Resolve(config.Toolchain)
	if err != nil {
		return r, err
	}
	if err := buildcfg.ValidateWorker(tc); err != nil {
		return r, err
	}
	if err := os.Setenv("PATH", filepath.Dir(tc.Executable)+string(filepath.ListSeparator)+os.Getenv("PATH")); err != nil {
		return r, err
	}
	// TODO(migration): Task 24 limits pending identities to the internal CI command.
	cfg, err := candidate.Resolve(config, flags.Options)
	if err != nil {
		return r, err
	}
	return ResolvedBuild{Config: config, Toolchain: tc, Identity: cfg.Identity, Selection: cfg.Selection}, nil
}
