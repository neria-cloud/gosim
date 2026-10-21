package spec

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SourceRevision binds resolved ancestry and conservative executable inputs.
// Provenance affects whole-source build identity, but never this adapter revision.
func SourceRevision(root, profileID string) (string, error) {
	defs, err := LoadDefinitions(os.DirFS(filepath.Join(root, "internal/compat/profiles")))
	if err != nil {
		return "", err
	}
	var resolved ResolvedFamily
	found := false
	for family := range defs {
		if "go"+strings.ReplaceAll(family, ".", "") != profileID {
			continue
		}
		resolved, err = ResolveDefinitions(defs, family)
		if err != nil {
			return "", err
		}
		found = true
		break
	}
	if !found {
		return "", fmt.Errorf("unknown profile %q", profileID)
	}
	canonical, err := resolved.Canonical()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	add := func(data []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(data)))
		h.Write(size[:])
		h.Write(data)
	}
	add([]byte("gosim-compat-source-v2"))
	add([]byte(resolved.Family))
	add(canonical)
	var paths []string
	for _, dir := range []string{"internal/compat", "internal/hostabi", "internal/buildspec"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(filename string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" || entry.Name() == ".gosim" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, filename)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			name := entry.Name()
			if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".md") || name == "revisions_gen.go" || rel == "internal/compat/profiles/evidence.json" || strings.HasPrefix(rel, "internal/compat/profiles/go") && strings.HasSuffix(name, ".origin.json") {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("nonregular compatibility input %s", rel)
			}
			paths = append(paths, rel)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	slices.Sort(paths)
	for _, name := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return "", err
		}
		add([]byte(name))
		add(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
