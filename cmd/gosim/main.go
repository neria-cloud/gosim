package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/candidate"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/gosimviewer"
	"github.com/neria-cloud/gosim/internal/translate"
)

//go:embed doc.go
var rawDoc string

func doc() string {
	doc := strings.TrimPrefix(rawDoc, "/*\n")
	doc = doc[:strings.LastIndex(doc, "*/")]
	return doc
}

func commandName(cmd string) string {
	return fmt.Sprintf("%s %s", path.Base(os.Args[0]), cmd)
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

	if err := copyFile(src, dst, perm); err != nil {
		return err
	}

	return nil
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

	if err := os.WriteFile(dst, b, perm); err != nil {
		return err
	}

	return nil
}

const toolsgoTemplate = `//go:build never

package main

import (
	_ "` + gosimtool.Module + `"
)
`

func main() {
	flag.Usage = func() { fmt.Print(doc()) }
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	command, arguments := flag.Arg(0), flag.Args()[1:]
	f := flag.NewFlagSet(commandName(command), flag.ExitOnError)
	if command == "viewer" {
		logs := f.String("log", "", "captured JSON log")
		source := f.String("source", "", "translated source directory from the matching artifact")
		f.Parse(arguments)
		if *source == "" {
			log.Fatal("viewer requires -source from the matching artifact")
		}
		gosimviewer.Viewer(*logs, *source)
		return
	}
	switch command {
	case "translate", "test", "build-tests", "debug", "prepare-selftest", "lookup-test", "resolve-build":
	default:
		flag.Usage()
		os.Exit(2)
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
	f.Parse(arguments)
	resolved, err := resolveBuild(command, *flags)
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := gosimtool.NewBuildConfig(resolved.Selection, resolved.Toolchain)
	if err != nil {
		log.Fatal(err)
	}
	patterns := f.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	switch command {
	case "resolve-build":
		if err := json.NewEncoder(os.Stdout).Encode(resolved.Identity); err != nil {
			log.Fatal(err)
		}
		return
	case "prepare-selftest":
		prepareSelftest(cfg)
		return
	case "translate":
		output, err := translate.Translate(&translate.TranslateInput{Packages: patterns, Cfg: cfg})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(output.RootOutputDir)
		return
	case "lookup-test":
		if len(patterns) != 1 {
			log.Fatal("lookup-test requires one package")
		}
		deps, err := translate.InspectDependencies(&translate.TranslateInput{Packages: patterns, Cfg: cfg})
		if err != nil {
			log.Fatal(err)
		}
		if len(deps) != 1 {
			log.Fatal("lookup-test requires one test package")
		}
		root, err := gosimtool.FindGoModDir()
		if err != nil {
			log.Fatal(err)
		}
		for p, sources := range deps {
			artifact, err := gosimtool.LookupArtifact(root, cfg.Identity, p, sources)
			if err != nil {
				log.Fatal(err)
			}
			if err := json.NewEncoder(os.Stdout).Encode(struct {
				Binary   string
				Sources  map[string]string
				Identity buildspec.Identity
			}{filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path), sources, cfg.Identity}); err != nil {
				log.Fatal(err)
			}
		}
		return
	case "debug":
		if *pkg == "" || *test == "" {
			log.Fatal("debug requires -package and -test")
		}
		patterns = []string{*pkg}
	}
	result, err := candidate.BuildResolved(cfg, patterns)
	if err != nil {
		log.Fatal(err)
	}
	if command == "build-tests" {
		return
	}
	for _, artifact := range result.Artifacts {
		binary := filepath.Join(artifact.Directory, artifact.Manifest.Binary.Path)
		args := []string{"-test.run=" + *run, "-test.skip=" + *skip, "-test.v=" + fmt.Sprint(*verbose), "-seeds=" + *seeds, "-simtrace=" + *trace, "-logformat=" + *format}
		if *jsonlog != "" {
			absolute, err := filepath.Abs(*jsonlog)
			if err != nil {
				log.Fatal(err)
			}
			args = append(args, "-jsonlogout="+absolute)
		}
		var child *exec.Cmd
		if command == "debug" {
			if len(result.Artifacts) != 1 {
				log.Fatal("debug requires one test package")
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
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stdout
		if err := child.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				os.Exit(exit.ExitCode())
			}
			log.Fatal(err)
		}
	}
}
