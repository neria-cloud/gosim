package buildcfg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// fingerprintInstalled identifies the installation Go selected. It does not
// download another copy or confuse installation trust with family compatibility.
func fingerprintInstalled(root string) (buildspec.Fingerprints, error) {
	return fingerprintInstalledContext(context.Background(), root)
}

func fingerprintInstalledContext(ctx context.Context, root string) (buildspec.Fingerprints, error) {
	var tools, sources []buildspec.Revision
	seen := make(map[string]bool)
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if entry.IsDir() {
			if name == "." || name == "src" || name == "bin" || name == "lib" || name == "pkg" || name == "pkg/tool" ||
				strings.HasPrefix(name, "src/") || strings.HasPrefix(name, "bin/") || strings.HasPrefix(name, "lib/") || strings.HasPrefix(name, "pkg/tool/") {
				return nil
			}
			return filepath.SkipDir
		}
		source := strings.HasPrefix(name, "src/")
		tool := name == "VERSION" || name == "go.env" || strings.HasPrefix(name, "bin/") || strings.HasPrefix(name, "lib/") || strings.HasPrefix(name, "pkg/tool/")
		if !source && !tool {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular GOROOT input %s", name)
		}
		digest, err := hashFile(filename)
		if err != nil {
			return err
		}
		item := buildspec.Revision{Name: name, Digest: digest}
		if source {
			sources = append(sources, item)
		} else {
			tools = append(tools, item)
		}
		seen[name] = true
		return nil
	})
	if err != nil {
		return buildspec.Fingerprints{}, err
	}
	for _, required := range []string{"VERSION", "bin/go", "src/runtime/runtime.go"} {
		if !seen[required] {
			return buildspec.Fingerprints{}, fmt.Errorf("incomplete Go installation: missing %s", required)
		}
	}
	// WalkDir gives lexical path order. Both inventories include paths and bytes.
	toolData, err := json.Marshal(tools)
	if err != nil {
		return buildspec.Fingerprints{}, err
	}
	sourceData, err := json.Marshal(sources)
	if err != nil {
		return buildspec.Fingerprints{}, err
	}
	return buildspec.Fingerprints{ToolchainDigest: buildspec.Hash(toolData), StdlibDigest: buildspec.Hash(sourceData)}, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() {
		return "", fmt.Errorf("nonregular input %s", path)
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", err
	}
	after, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if n != before.Size() || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return "", fmt.Errorf("input changed while hashing: %s", path)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
