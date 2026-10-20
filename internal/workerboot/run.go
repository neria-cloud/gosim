package workerboot

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

type Streams struct {
	In       io.Reader
	Out, Err io.Writer
}

func Run(ctx context.Context, w Worker, r buildspec.Resolution, args []string, streams Streams) (int, error) {
	if err := Verify(ctx, w, r); err != nil {
		return 1, err
	}
	file, err := os.CreateTemp("", "gosim-resolution-*.json")
	if err != nil {
		return 1, err
	}
	defer os.Remove(file.Name())
	if err := json.NewEncoder(file).Encode(Invocation{Schema: 1, Resolution: r, Manifest: w.Manifest}); err != nil {
		file.Close()
		return 1, err
	}
	if err := file.Close(); err != nil {
		return 1, err
	}
	command := append([]string{"--resolution", file.Name()}, args...)
	// #nosec G204 -- the selected worker is validated before this entry point runs it.
	cmd := exec.CommandContext(ctx, w.Path, command...)
	cmd.Dir = r.Context.Directory
	cmd.Env = buildspec.ApplicationCommand(ctx, r, "env").Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = streams.In, streams.Out, streams.Err
	err = buildspec.RunProcess(ctx, cmd)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return buildspec.ExitCode(err), nil
	}
	return buildspec.ExitCode(err), err
}

const Help = `Gosim translates Go tests and runs them in a deterministic simulation.

Usage: gosim [-C directory] <command> [options] [packages]
Commands: test, translate, build-tests, prepare-selftest, debug, viewer
Go tooling selects the compiler from the caller's module/workspace.
-C directory runs in that directory and must precede the command.
Optional overrides: -go go1.X.Y, -config path, -features rt[,fs][,net], -race
Certification is a separate, explicitly invoked manual command.
`

// startupDirectory resolves the launch context without changing the process's
// working directory. Discovery and the worker both use this directory.
func startupDirectory(args []string) (string, []string, error) {
	if len(args) == 0 || args[0] != "-C" && !strings.HasPrefix(args[0], "-C=") {
		return "", args, nil
	}
	directory := strings.TrimPrefix(args[0], "-C=")
	rest := args[1:]
	if args[0] == "-C" {
		if len(rest) == 0 {
			return "", nil, fmt.Errorf("missing directory for -C")
		}
		directory, rest = rest[0], rest[1:]
	}
	if directory == "" {
		return "", nil, fmt.Errorf("empty directory for -C")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", nil, fmt.Errorf("-C: %w", err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		return "", nil, fmt.Errorf("-C: %w", err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("-C: %s is not a directory", directory)
	}
	return directory, rest, nil
}

// startupOverrides reads only startup options, respecting flag values and the
// first positional package. Full command validation belongs to the worker.
func startupOverrides(args []string) (config, version string, err error) {
	if len(args) == 0 {
		return "", "", nil
	}
	start := 1
	if args[0] == "internal-candidate" {
		start = 2
		if len(args) < 2 {
			return "", "", fmt.Errorf("internal-candidate requires an operation")
		}
	}
	booleans := map[string]bool{
		"race":     true,
		"v":        true,
		"headless": true,
		"json":     true,
		"ordinary": true,
		"large":    true,
	}
	for i := start; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			break
		}
		name, value, has := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if name == "h" || name == "help" {
			return "", "", flag.ErrHelp
		}
		if name == "resolution" || name == "worker-info" {
			return "", "", fmt.Errorf("private worker flags are not launcher options")
		}
		if name == "C" {
			return "", "", fmt.Errorf("-C must precede the command")
		}
		if !has && !booleans[name] {
			i++
			if i == len(args) {
				return "", "", fmt.Errorf("missing value for -%s", name)
			}
			value = args[i]
		}
		switch name {
		case "go":
			version = value
		case "config":
			config = value
		}
	}
	return config, version, nil
}

// ConfigPath locates optional caller configuration without requiring a nearest module.
func ConfigPath(directory, explicit string) (string, error) {
	configPath := explicit
	if configPath == "" {
		configPath = os.Getenv("GOSIM_CONFIG")
	}
	if configPath != "" {
		if filepath.IsAbs(configPath) {
			return configPath, nil
		}
		return filepath.Join(directory, configPath), nil
	}
	// Configuration follows the nearest caller module; Go itself resolves workspaces.
	for dir := directory; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			path := filepath.Join(dir, "gosim.json")
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return "", nil
			} else if err != nil {
				return "", err
			}
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if filepath.Dir(dir) == dir {
			return "", nil
		}
	}
}

func Main(ctx context.Context, args []string, streams Streams) int {
	fail := func(err error) int { fmt.Fprintln(streams.Err, "gosim:", err); return buildspec.ExitCode(err) }
	cwd, remainingArgs, err := startupDirectory(args)
	if err != nil {
		return fail(err)
	}
	if len(remainingArgs) == 0 || remainingArgs[0] == "help" ||
		remainingArgs[0] == "-h" || remainingArgs[0] == "--help" {
		fmt.Fprint(streams.Out, Help)
		return 0
	}
	if remainingArgs[0] == "version" || remainingArgs[0] == "--version" {
		fmt.Fprintln(streams.Out, "gosim launcher; worker protocol", Protocol)
		return 0
	}
	path, version, err := startupOverrides(remainingArgs)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(streams.Out, Help)
		return 0
	}
	if err != nil {
		return fail(err)
	}
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return fail(err)
		}
	}
	path, err = ConfigPath(cwd, path)
	if err != nil {
		return fail(err)
	}
	overrides := buildcfg.Overrides{}
	if version != "" {
		overrides.Toolchain = &version
	}
	request, err := buildcfg.Load(path, overrides)
	if err != nil {
		return fail(err)
	}
	r, err := buildcfg.Discover(ctx, cwd, request.ExplicitToolchain)
	if err != nil {
		return fail(err)
	}
	if err := buildspec.CheckEnvironment(r.Toolchain); err != nil {
		return fail(err)
	}
	inputs, err := buildcfg.Fingerprint(r.Toolchain)
	if err != nil {
		return fail(err)
	}
	source, err := ResolveSource(ctx, r)
	if err != nil {
		return fail(err)
	}
	worker, err := Ensure(ctx, r, source, inputs, "")
	if err != nil {
		return fail(err)
	}
	code, err := Run(ctx, worker, r, remainingArgs, streams)
	if err != nil {
		return fail(err)
	}
	return code
}
