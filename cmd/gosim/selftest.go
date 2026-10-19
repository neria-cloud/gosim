package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

// prepareSelftest prepares for running tests in the gosim module.
func prepareSelftest(cfg gosimtool.BuildConfig) {
	modDir, err := gosimtool.FindGoModDir()
	if err != nil {
		log.Fatal(err)
	}

	buildDir, err := os.MkdirTemp("", "gosimbuild")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(buildDir)

	// Prepare the racetest binary
	raceBuildPath := filepath.Join(buildDir, "testdata.test")
	raceFinalPath := filepath.Join(modDir, gosimtool.OutputDirectory, "racetest", "testdata.test")
	if err := os.MkdirAll(filepath.Dir(raceFinalPath), 0o755); err != nil {
		log.Fatal(err)
	}
	// try to reuse the old binary
	if err := copyFile(raceFinalPath, raceBuildPath, 0o755); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Fatalf("copying source: %s", err)
		}
	}
	// do the build
	args := []string{"test"}
	args = append(args, "-ldflags=-checklinkname=0", "-tags=linkname")
	args = append(args, "-race")
	args = append(args, "-c")
	args = append(args, "-o", buildDir)
	args = append(args, "./internal/tests/race/testdata")
	cmd := buildcfg.Command(cfg.Toolchain, modDir, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatal(err)
	}
	// place the binary
	if err := copyFileIfChanged(raceBuildPath, raceFinalPath, 0o755); err != nil {
		log.Fatalf("copying final: %s", err)
	}

	// Prepare the gosim binary
	// TODO: should we have a -race version as well?
	gosimBuildPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	gosimFinalPath := filepath.Join(modDir, gosimtool.OutputDirectory, "scripttest", "bin", "gosim")
	if err := os.MkdirAll(filepath.Dir(gosimFinalPath), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := copyFileIfChanged(gosimBuildPath, gosimFinalPath, 0o755); err != nil {
		log.Fatalf("copying final: %s", err)
	}

	// Prepare the module
	copyModuleForScriptTest(modDir)
}

// copyModuleForScriptTest copies the source code of this module to a slimmed-down
// version for the script tests in ./internal/tests to make them interact nicely
// with the go test cache.
func copyModuleForScriptTest(modDir string) {
	var files []string
	wanted := make(map[string]bool)
	err := filepath.WalkDir(modDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(modDir, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".gosim" || rel == "docs" {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == ".git" || filepath.Ext(rel) == ".md" {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular source input %s", path)
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	extractedModDir := filepath.Join(modDir, gosimtool.OutputDirectory, "scripttest/mod")

	for _, file := range files {
		newP := filepath.Join(extractedModDir, strings.TrimPrefix(file, modDir))
		if err := os.MkdirAll(filepath.Dir(newP), 0o755); err != nil {
			log.Fatal(err)
		}
		if err := copyFileIfChanged(file, newP, 0o644); err != nil {
			log.Fatal(err)
		}
		wanted[newP] = true
	}

	err = filepath.WalkDir(extractedModDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !wanted[path] {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// TODO: delete unwanted directories?
}
