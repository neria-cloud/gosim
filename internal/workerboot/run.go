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

Usage: gosim <command> [options] [packages]
Commands: test, translate, build-tests, prepare-selftest, debug, viewer
Go tooling selects the compiler from the caller's module/workspace.
Optional overrides: -go go1.X.Y, -config path, -features rt[,fs][,net], -race
Certification is a separate, explicitly invoked manual command.
`

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
	booleans := map[string]bool{"race": true, "v": true, "headless": true, "json": true, "ordinary": true, "large": true}
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

func configPath(directory, explicit string) (string, error) {
	if explicit == "" {
		explicit = os.Getenv("GOSIM_CONFIG")
	}
	if explicit != "" {
		if filepath.IsAbs(explicit) {
			return explicit, nil
		}
		return filepath.Join(directory, explicit), nil
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
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(streams.Out, Help)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(streams.Out, "gosim launcher; worker protocol", Protocol)
		return 0
	}
	fail := func(err error) int { fmt.Fprintln(streams.Err, "gosim:", err); return buildspec.ExitCode(err) }
	path, version, err := startupOverrides(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(streams.Out, Help)
		return 0
	}
	if err != nil {
		return fail(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err)
	}
	path, err = configPath(cwd, path)
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
	code, err := Run(ctx, worker, r, args, streams)
	if err != nil {
		return fail(err)
	}
	return code
}
