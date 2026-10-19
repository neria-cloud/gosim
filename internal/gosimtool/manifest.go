package gosimtool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type PackageDigest struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}
type ArtifactFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Schema         int                `json:"schema"`
	Identity       buildspec.Identity `json:"identity"`
	PackageDigests []PackageDigest    `json:"package_digests"`
	Binary         ArtifactFile       `json:"binary"`
	GeneratedFiles []ArtifactFile     `json:"generated_files"`
}
type Artifact struct {
	Directory string
	Manifest  Manifest
}

func ValidateArtifactPath(name string) error {
	if name == "" || name == "." || path.IsAbs(name) || path.Clean(name) != name || strings.ContainsAny(name, "\\:\x00") || name == ".." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("invalid relative artifact path %q", name)
	}
	return nil
}

// ValidateGeneratedFiles validates a complete materialization set before I/O.
func ValidateGeneratedFiles(files map[string][]byte) error {
	names := map[string]bool{"manifest.json": true}
	for name := range files {
		if err := ValidateArtifactPath(name); err != nil {
			return err
		}
		if names[name] {
			return fmt.Errorf("reserved output path %q", name)
		}
		names[name] = true
	}
	for name := range names {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if names[parent] {
				return fmt.Errorf("output file/directory conflict %q", name)
			}
		}
	}
	return nil
}

func normalizePackages(packages []PackageDigest) ([]PackageDigest, error) {
	result := slices.Clone(packages)
	slices.SortFunc(result, func(a, b PackageDigest) int { return strings.Compare(a.Path, b.Path) })
	if len(result) == 0 {
		return nil, fmt.Errorf("empty artifact package set")
	}
	for i, p := range result {
		if p.Path == "" || strings.ContainsAny(p.Path, "\x00\r\n") || !buildspec.ValidDigest(p.Digest) || i > 0 && result[i-1].Path == p.Path {
			return nil, fmt.Errorf("invalid/duplicate package digest %q", p.Path)
		}
	}
	return result, nil
}

func PackageBuildDigest(packages []PackageDigest) (string, error) {
	normalized, err := normalizePackages(packages)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return buildspec.Hash(data), nil
}

func normalizeManifest(m Manifest) (Manifest, error) {
	if m.Schema != 1 {
		return Manifest{}, fmt.Errorf("unsupported manifest schema %d", m.Schema)
	}
	var err error
	m.Identity, err = buildspec.NormalizeIdentity(m.Identity)
	if err != nil {
		return Manifest{}, err
	}
	m.PackageDigests, err = normalizePackages(m.PackageDigests)
	if err != nil {
		return Manifest{}, err
	}
	m.GeneratedFiles = slices.Clone(m.GeneratedFiles)
	if m.GeneratedFiles == nil {
		m.GeneratedFiles = []ArtifactFile{}
	}
	slices.SortFunc(m.GeneratedFiles, func(a, b ArtifactFile) int { return strings.Compare(a.Path, b.Path) })
	names := map[string]bool{"manifest.json": true}
	for _, f := range append([]ArtifactFile{m.Binary}, m.GeneratedFiles...) {
		if err := ValidateArtifactPath(f.Path); err != nil {
			return Manifest{}, err
		}
		if !buildspec.ValidDigest(f.SHA256) || names[f.Path] {
			return Manifest{}, fmt.Errorf("invalid/duplicate artifact file %q", f.Path)
		}
		names[f.Path] = true
	}
	for name := range names {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if names[parent] {
				return Manifest{}, fmt.Errorf("artifact file/directory conflict %q", name)
			}
		}
	}
	return m, nil
}

func CanonicalManifest(m Manifest) ([]byte, error) {
	m, err := normalizeManifest(m)
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func decodeManifest(data []byte) (Manifest, error) {
	var m Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Manifest{}, fmt.Errorf("trailing manifest data")
	}
	return normalizeManifest(m)
}

func ArtifactDirectory(project string, identity buildspec.Identity, packages []PackageDigest) (string, error) {
	if project == "" {
		return "", fmt.Errorf("explicit project directory is required")
	}
	build, err := buildspec.Digest(identity)
	if err != nil {
		return "", err
	}
	pkg, err := PackageBuildDigest(packages)
	if err != nil {
		return "", err
	}
	return filepath.Join(project, OutputDirectory, "v2", build, "packages", pkg), nil
}

// regularArtifactFile rejects symlinks at every relative path component. The
// compiler writes into private staging; readers never follow an unrecorded link.
func regularArtifactFile(directory, name string) (string, error) {
	if err := ValidateArtifactPath(name); err != nil {
		return "", err
	}
	root, err := os.Lstat(directory)
	if err != nil {
		return "", err
	}
	if !root.IsDir() {
		return "", fmt.Errorf("artifact root is not a directory")
	}
	current := directory
	parts := strings.Split(name, "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return "", fmt.Errorf("artifact parent is not a directory: %s", name)
			}
		} else if !info.Mode().IsRegular() {
			return "", fmt.Errorf("artifact is not a regular file: %s", name)
		}
	}
	return current, nil
}

func verifyManifestInputs(m Manifest, identity buildspec.Identity, packages []PackageDigest) error {
	want, err := buildspec.Digest(identity)
	if err != nil {
		return err
	}
	got, err := buildspec.Digest(m.Identity)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("artifact build identity mismatch")
	}
	want, err = PackageBuildDigest(packages)
	if err != nil {
		return err
	}
	got, err = PackageBuildDigest(m.PackageDigests)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("artifact package content digest mismatch")
	}
	return nil
}

func verifyManifestFiles(directory string, m Manifest) error {
	expected := map[string]bool{"manifest.json": true}
	for _, f := range append([]ArtifactFile{m.Binary}, m.GeneratedFiles...) {
		file, err := regularArtifactFile(directory, f.Path)
		if err != nil {
			return fmt.Errorf("verify artifact %s: %w", f.Path, err)
		}
		if f.Path == m.Binary.Path {
			info, err := os.Stat(file)
			if err != nil {
				return err
			}
			if info.Mode().Perm()&0o111 == 0 {
				return fmt.Errorf("artifact binary is not executable: %s", f.Path)
			}
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if buildspec.Hash(data) != f.SHA256 {
			return fmt.Errorf("artifact content mismatch: %s", f.Path)
		}
		expected[f.Path] = true
	}
	// An extra generated source could alter a future compile; every file belongs
	// in the manifest. Never admit symlinks or other filesystem object types.
	return filepath.WalkDir(directory, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(directory, file)
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() || !expected[filepath.ToSlash(rel)] {
			return fmt.Errorf("unrecorded/nonregular artifact file %s", rel)
		}
		return nil
	})
}

func VerifyArtifact(directory string, identity buildspec.Identity, packages []PackageDigest) (Manifest, error) {
	file, err := regularArtifactFile(directory, "manifest.json")
	if err != nil {
		return Manifest{}, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return Manifest{}, err
	}
	m, err := decodeManifest(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("read artifact manifest: %w", err)
	}
	if err := verifyManifestInputs(m, identity, packages); err != nil {
		return Manifest{}, err
	}
	if err := verifyManifestFiles(directory, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}
