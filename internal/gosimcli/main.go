package gosimcli

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	stdpath "path"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildrun"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/gosimviewer"
	"github.com/neria-cloud/gosim/internal/translate"
	"github.com/neria-cloud/gosim/internal/workerboot"
)

//go:embed doc.go
var rawDoc string

func doc() string {
	doc := strings.TrimPrefix(rawDoc, "/*\n")
	doc = doc[:strings.LastIndex(doc, "*/")]
	return doc
}

func commandName(cmd string) string {
	return fmt.Sprintf("%s %s", stdpath.Base(os.Args[0]), cmd)
}

func packageName(pkgPath string) string {
	return pkgPath[strings.LastIndex(pkgPath, "/")+1:]
}

func batchPackagesWithDifferentNames(packages []string) [][]string {
	var groups [][]string
	count := make(map[string]int)

	for _, pkg := range packages {
		lastName := packageName(pkg)
		group := count[lastName]
		count[lastName]++
		if group >= len(groups) {
			groups = append(groups, nil)
		}
		groups[group] = append(groups[group], pkg)
	}

	return groups
}

func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func copyFile(from, to string, perm fs.FileMode) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	// Publish on a fresh inode. Rewriting an executed Mach-O file in place can
	// leave macOS's cached code signature invalid even when codesign verifies its
	// bytes on disk. A failed copy must also preserve the complete old artifact.
	dst, err := os.CreateTemp(filepath.Dir(to), ".gosim-copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(dst.Name())
	defer dst.Close()
	if err := dst.Chmod(perm); err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Rename(dst.Name(), to)
}

func copyFileIfChanged(src, dst string, perm fs.FileMode) error {
	// skip unchanged so we keep the old modified timestamp
	dstHash, err := hashFile(dst)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if dstHash != nil {
		srcHash, err := hashFile(src)
		if err != nil {
			return err
		}
		if bytes.Equal(srcHash, dstHash) {
			// skip because unchanged, maybe log?
			return nil
		}
	}

	return copyFile(src, dst, perm)
}

func writeFileIfDifferent(b []byte, dst string, perm fs.FileMode) error {
	// skip unchanged so we keep the old modified timestamp
	dstBytes, err := os.ReadFile(dst)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if bytes.Equal(b, dstBytes) {
		// skip because unchanged, maybe log?
		return nil
	}

	return os.WriteFile(dst, b, perm)
}

const toolsgoTemplate = `//go:build never

package main

import (
	_ "` + gosimtool.Module + `"
)
`

// Run is the direct worker entry. It never bootstraps another worker.
func Run(ctx context.Context, args []string, streams workerboot.Streams) int {
	if len(args) >= 2 && args[0] == "--resolution" {
		r, err := workerboot.ValidateInvocation(ctx, args[1])
		if err != nil {
			fmt.Fprintln(streams.Err, err)
			return 1
		}
		ctx = context.WithValue(ctx, resolutionKey{}, r)
		args = args[2:]
	}
	code, err := run(ctx, args, streams)
	if err != nil {
		fmt.Fprintln(streams.Err, "gosim:", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

func run(ctx context.Context, args []string, streams workerboot.Streams) (int, error) {
	if len(args) == 0 {
		fmt.Fprint(streams.Out, doc())
		return 2, nil
	}
	command, arguments := args[0], args[1:]
	internal := command == "internal-candidate"
	if internal {
		if len(arguments) == 0 {
			return 1, fmt.Errorf("internal-candidate requires an operation")
		}
		command, arguments = arguments[0], arguments[1:]
	}
	if internal && (command == "verify" || command == "certify") {
		return runCertificationCommand(ctx, command, arguments, streams)
	}

	f := flag.NewFlagSet(commandName(command), flag.ContinueOnError)
	f.SetOutput(streams.Err)
	if command == "viewer" {
		logs := f.String("log", "", "captured JSON log")
		source := f.String("source", "", "translated source directory from the matching artifact")
		if err := f.Parse(arguments); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0, nil
			}
			return 2, err
		}
		if *source == "" {
			return 1, fmt.Errorf("viewer requires -source from the matching artifact")
		}
		gosimviewer.Viewer(*logs, *source)
		return 0, nil
	}
	switch command {
	case "translate", "test", "build-tests", "debug", "prepare-selftest", "lookup-test", "resolve-build":
	default:
		fmt.Fprint(streams.Err, doc())
		return 2, fmt.Errorf("unknown command %q", command)
	}
	flags := addBuildFlags(f)
	verbose := f.Bool("v", false, "verbose test output")
	run := f.String("run", "", "test match expression")
	skip := f.String("skip", "", "test skip expression")
	seeds := f.String("seeds", "1", "seeds and ranges")
	trace := f.String("simtrace", "", "comma-separated trace categories")
	format := f.String("logformat", "pretty", "raw|indented|pretty")
	jsonlog := f.String("jsonlogout", "", "captured JSON log path")
	pkg := f.String("package", "", "package to debug")
	test := f.String("test", "", "test to debug")
	seed := f.Int("seed", 1, "debug seed")
	step := f.Int("step", 0, "debug breakpoint step")
	headless := f.Bool("headless", false, "headless debugger")
	if err := f.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}
	resolveCommand := command
	if internal {
		resolveCommand = "internal-candidate"
	}
	resolved, err := resolveBuildForContext(ctx, resolveCommand, *flags)
	if err != nil {
		return 1, err
	}
	cfg, err := gosimtool.NewBuildConfig(ctx, resolved.Selection, resolved.Resolution)
	if err != nil {
		return 1, err
	}
	patterns := f.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	switch command {
	case "resolve-build":
		if err := json.NewEncoder(streams.Out).Encode(resolved.Identity); err != nil {
			return 1, err
		}
		return 0, nil
	case "prepare-selftest":
		if err := prepareSelftest(cfg); err != nil {
			return 1, err
		}
		return 0, nil
	case "translate":
		output, err := translate.Translate(&translate.TranslateInput{Packages: patterns, Cfg: cfg})
		if err != nil {
			return 1, err
		}
		fmt.Fprintln(streams.Out, output.RootOutputDir)
		return 0, nil
	case "lookup-test":
		if len(patterns) != 1 {
			return 1, fmt.Errorf("lookup-test requires one package")
		}
		deps, err := translate.InspectDependencies(&translate.TranslateInput{Packages: patterns, Cfg: cfg})
		if err != nil {
			return 1, err
		}
		if len(deps) != 1 {
			return 1, fmt.Errorf("lookup-test requires one test package")
		}
		root, err := cfg.ProjectRoot()
		if err != nil {
			return 1, err
		}
		for p, sources := range deps {
			artifact, err := gosimtool.LookupArtifact(root, cfg.Identity, p, sources)
			if err != nil {
				return 1, err
			}
			if err := json.NewEncoder(streams.Out).Encode(struct {
				Binary   string
				Sources  map[string]string
				Identity buildspec.Identity
			}{filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path), sources, cfg.Identity}); err != nil {
				return 1, err
			}
		}
		return 0, nil
	case "debug":
		if *pkg == "" || *test == "" {
			return 1, fmt.Errorf("debug requires -package and -test")
		}
		patterns = []string{*pkg}
	}
	result, err := buildrun.BuildResolved(cfg, patterns)
	if err != nil {
		return 1, err
	}
	if command == "build-tests" {
		return 0, nil
	}
	for _, artifact := range result.Artifacts {
		binary := filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path)
		args := []string{"-test.run=" + *run, "-test.skip=" + *skip, "-test.v=" + fmt.Sprint(*verbose), "-seeds=" + *seeds, "-simtrace=" + *trace, "-logformat=" + *format}
		if *jsonlog != "" {
			absolute, err := filepath.Abs(*jsonlog)
			if err != nil {
				return 1, err
			}
			args = append(args, "-jsonlogout="+absolute)
		}
		var child *exec.Cmd
		if command == "debug" {
			if len(result.Artifacts) != 1 {
				return 1, fmt.Errorf("debug requires one test package")
			}
			dlv := []string{"exec", binary}
			if *headless {
				dlv = append(dlv, "--headless", "--listen=:2345", "--accept-multiclient")
			}
			dlv = append(dlv, "--", "-test.run=^"+*test+"$", "-test.v", "-seeds="+fmt.Sprint(*seed), "-step-breakpoint="+fmt.Sprint(*step))
			child = exec.Command("dlv", dlv...)
		} else {
			child = exec.Command(binary, args...)
		}
		child.Env = append(os.Environ(), "GODEBUG=")
		// Preserve the CLI test-output contract: simulation diagnostics and logs
		// form one stdout stream. Launcher/worker diagnostics still use stderr.
		child.Stdin, child.Stdout, child.Stderr = streams.In, streams.Out, streams.Out
		if err := buildspec.RunProcess(ctx, child); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return buildspec.ExitCode(err), nil
			}
			return 1, err
		}
	}
	return 0, nil
}
