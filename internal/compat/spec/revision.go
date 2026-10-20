package spec

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// SourceRevision fingerprints executable adapters, manifests, source origins and
// policies. Tests and certification results are excluded: recording evidence must
// not change the source identity that evidence certifies. File paths are hashed
// as well as contents, so moving/deleting an adapter also invalidates identity.
func SourceRevision(root, family string) (string, error) {
	if _, err := buildspec.LookupProfile(family); err != nil {
		return "", fmt.Errorf("unknown profile %q", family)
	}
	var paths []string
	for _, dir := range []string{"internal/compat/" + family, "internal/hostabi", "internal/compat/selected", "internal/compat/spec", "internal/buildspec"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			name := entry.Name()
			if strings.HasSuffix(name, "_test.go") ||
				name == "revision.go" &&
					strings.Contains(path, string(filepath.Separator)+"profile"+string(filepath.Separator)) ||
				name == "tuples.json" ||
				name == "evidence.json" &&
					strings.Contains(path, string(filepath.Separator)+"profile"+string(filepath.Separator)) {
				return nil
			}
			if !strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".json") {
				return nil
			}
			// Shared facade selection includes only this family's implementation.
			for _, other := range buildspec.SupportedFamilies() {
				if other.Profile != family && strings.Contains(name, other.Profile) {
					return nil
				}
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	slices.Sort(paths)
	records := make([]buildspec.Revision, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return "", err
		}
		records = append(records, buildspec.Revision{Name: path, Digest: buildspec.Hash(data)})
	}
	data, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	return buildspec.Hash(data), nil
}
