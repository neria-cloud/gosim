package spec

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// LoadOrigin reads a profiles directory, independently of runtime support admission.
// Callers supply the checkout explicitly; provenance never comes from an embed.
func LoadOrigin(fsys fs.FS, profileID string) (Origin, error) {
	var origin Origin
	if !strings.HasPrefix(profileID, "go1") {
		return origin, fmt.Errorf("invalid profile ID %q", profileID)
	}
	family := "1." + strings.TrimPrefix(profileID, "go1")
	if _, err := familyMinor(family); err != nil {
		return origin, err
	}
	data, err := fs.ReadFile(fsys, profileID+".origin.json")
	if err != nil {
		return origin, err
	}
	if err := json.Unmarshal(data, &origin); err != nil {
		return origin, err
	}
	version, err := buildspec.ParseGoVersion(origin.BaselineToolchain)
	if err != nil || origin.Schema != 2 || origin.Family != family || version.Family() != family || len(origin.Files) == 0 {
		return Origin{}, fmt.Errorf("invalid origin metadata for %s", profileID)
	}
	seen := map[string]bool{}
	for _, file := range origin.Files {
		if !fs.ValidPath(file.Path) || !strings.HasPrefix(file.Path, "src/") || strings.Contains(file.Path, "\\") || seen[file.Path] || !buildspec.ValidDigest(file.Digest) {
			return Origin{}, fmt.Errorf("invalid origin source %q", file.Path)
		}
		seen[file.Path] = true
	}
	return origin, nil
}
