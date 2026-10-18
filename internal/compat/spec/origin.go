package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// Origin records the exact upstream files reviewed for private ABI access. It is
// source provenance, not execution/certification evidence. Distribution validation
// remains the responsibility of buildcfg.Fingerprint.
type Origin struct {
	Toolchain string       `json:"toolchain"`
	Files     []SourceFile `json:"files"`
}

type SourceFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	// Native declarations include race-only files that VerifyHooks' Linux/non-race
	// source selection intentionally does not load.
	Hooks []Hook `json:"hooks,omitempty"`
}

func VerifyOrigin(tc buildspec.Toolchain, origin Origin) error {
	if tc.Version != origin.Toolchain || len(origin.Files) == 0 {
		return fmt.Errorf("native ABI origin requires %s, got %s", origin.Toolchain, tc.Version)
	}
	output, err := buildspec.GoCommand(tc, "", "version").Output()
	if err != nil || !strings.HasPrefix(string(output), "go version "+origin.Toolchain+" ") {
		return fmt.Errorf("native ABI executable does not match %s: %s (%v)", origin.Toolchain, output, err)
	}
	version, err := os.ReadFile(filepath.Join(tc.GOROOT, "VERSION"))
	if err != nil || strings.SplitN(string(version), "\n", 2)[0] != origin.Toolchain {
		return fmt.Errorf("native ABI GOROOT does not match %s", origin.Toolchain)
	}
	seen := make(map[string]bool)
	for _, file := range origin.Files {
		if !strings.HasPrefix(file.Path, "src/") || filepath.ToSlash(filepath.Clean(file.Path)) != file.Path || strings.Contains(file.Path, "\\") || seen[file.Path] || !buildspec.ValidDigest(file.Digest) {
			return fmt.Errorf("invalid native ABI source %q", file.Path)
		}
		seen[file.Path] = true
		filename := filepath.Join(tc.GOROOT, filepath.FromSlash(file.Path))
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("native ABI source %s: %w", file.Path, err)
		}
		if buildspec.Hash(data) != file.Digest {
			return fmt.Errorf("native ABI source modified: %s", file.Path)
		}
		for _, hook := range file.Hooks {
			symbols := make(map[string]string)
			if err := readHookDeclarations(filename, hook.Package, symbols); err != nil {
				return err
			}
			if got := symbols[hook.Package+"."+hook.Symbol]; got != hook.Signature {
				return fmt.Errorf("native ABI declaration drift for %s.%s: got %q, expected %q", hook.Package, hook.Symbol, got, hook.Signature)
			}
		}
	}
	return nil
}
