package gosimcli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

// prepareSelftest prepares for running tests in the gosim module.
func prepareSelftest(cfg gosimtool.BuildConfig) error {
	modDir, err := gosimtool.FindGoModDir()
	if err != nil {
		return err
	}

	// The native race oracle is needed only for an explicitly selected race run.
	if cfg.Identity.Race {
		buildDir, err := os.MkdirTemp("", "gosimbuild")
		if err != nil {
			return err
		}
		defer os.RemoveAll(buildDir)

		// Prepare the racetest binary
		raceBuildPath := filepath.Join(buildDir, "testdata.test")
		raceFinalPath := filepath.Join(modDir, gosimtool.OutputDirectory, "racetest", "testdata.test")
		if err := os.MkdirAll(filepath.Dir(raceFinalPath), 0o755); err != nil {
			return err
		}
		// try to reuse the old binary
		if err := copyFile(raceFinalPath, raceBuildPath, 0o755); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("copy source: %w", err)
			}
		}
		// do the build
		args := []string{"test"}
		args = append(args, "-ldflags=-checklinkname=0", "-tags=linkname")
		args = append(args, "-race")
		args = append(args, "-c")
		args = append(args, "-o", buildDir)
		args = append(args, "./internal/tests/race/testdata")
		cmd := buildspec.OutputCommand(cfg.Context, cfg.Toolchain, modDir, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := buildspec.RunProcess(cfg.Context, cmd); err != nil {
			return err
		}
		// place the binary
		if err := copyFileIfChanged(raceBuildPath, raceFinalPath, 0o755); err != nil {
			return fmt.Errorf("copy final: %w", err)
		}

		if err := gosimtool.RecordNativeArtifact(raceFinalPath, cfg.Identity); err != nil {
			return err
		}

	}

	// Script callers must perform their own normal Go/source selection. Copying
	// this worker would bind every fixture to the preparation process's compiler.
	buildDir, err := os.MkdirTemp("", "gosim-script-launcher-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(buildDir)
	gosimBuildPath := filepath.Join(buildDir, "gosim")
	cmd := buildspec.ApplicationCommand(cfg.Context, cfg.Resolution, "build", "-ldflags=-checklinkname=0", "-tags=linkname", "-o", gosimBuildPath, gosimtool.Module+"/cmd/gosim")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := buildspec.RunProcess(cfg.Context, cmd); err != nil {
		return err
	}
	gosimFinalPath := filepath.Join(modDir, gosimtool.OutputDirectory, "scripttest", "bin", "gosim-user")
	if err := os.MkdirAll(filepath.Dir(gosimFinalPath), 0o755); err != nil {
		return err
	}
	if err := copyFileIfChanged(gosimBuildPath, gosimFinalPath, 0o755); err != nil {
		return fmt.Errorf("copy final: %w", err)
	}

	wrapper := []byte("#!/bin/sh\nexec " + "'" + strings.ReplaceAll(gosimFinalPath, "'", "'\\''") + "'" + " internal-candidate \"$@\"\n")
	if err := os.WriteFile(filepath.Join(filepath.Dir(gosimFinalPath), "gosim"), wrapper, 0o755); err != nil {
		return err
	}

	// Prepare the module
	return copyModuleForScriptTest(modDir)
}

// copyModuleForScriptTest copies the source code of this module to a slimmed-down
// version for the script tests in ./internal/tests to make them interact nicely
// with the go test cache.
func copyModuleForScriptTest(modDir string) error {
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
			if entry.Name() == ".git" || entry.Name() == ".gosim" || rel == "docs" || rel == "test-reports" {
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
		return err
	}

	extractedModDir := filepath.Join(modDir, gosimtool.OutputDirectory, "scripttest/mod")

	for _, file := range files {
		newP := filepath.Join(extractedModDir, strings.TrimPrefix(file, modDir))
		if err := os.MkdirAll(filepath.Dir(newP), 0o755); err != nil {
			return err
		}
		if err := copyFileIfChanged(file, newP, 0o644); err != nil {
			return err
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
		return err
	}

	return nil
}
