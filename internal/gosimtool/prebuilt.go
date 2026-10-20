package gosimtool

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// PackageSourceDigest binds a translated package name to its complete, currently
// selected source closure. Callers must reload that closure with the pinned
// loader; checking only files recorded by an old build would miss added sources.
func PackageSourceDigest(pkg string, sources map[string]string) (PackageDigest, error) {
	if len(sources) == 0 {
		return PackageDigest{}, fmt.Errorf("empty source closure for %q", pkg)
	}
	for path, digest := range sources {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || !buildspec.ValidDigest(digest) {
			return PackageDigest{}, fmt.Errorf("invalid source input %q for %q", path, pkg)
		}
	}
	data, err := json.Marshal(struct {
		Package string
		Sources map[string]string
	}{pkg, sources})
	if err != nil {
		return PackageDigest{}, err
	}
	result := PackageDigest{Path: pkg, Digest: buildspec.Hash(data)}
	if _, err := normalizePackages([]PackageDigest{result}); err != nil {
		return PackageDigest{}, err
	}
	return result, nil
}

// LookupArtifact never chooses the newest binary or scans another identity's
// namespace. The selected current inputs determine one exact artifact slot.
func LookupArtifact(
	project string,
	identity buildspec.Identity,
	pkg string,
	sources map[string]string,
) (Artifact, error) {
	input, err := PackageSourceDigest(pkg, sources)
	if err != nil {
		return Artifact{}, err
	}
	inputs := []PackageDigest{input}
	directory, err := ArtifactDirectory(project, identity, inputs)
	if err != nil {
		return Artifact{}, err
	}
	relative, err := filepath.Rel(project, filepath.Join(directory, "manifest.json"))
	if err != nil {
		return Artifact{}, err
	}
	if _, err := regularArtifactFile(project, filepath.ToSlash(relative)); err != nil {
		return Artifact{}, fmt.Errorf(
			"prebuilt %s unavailable for the selected identity and current sources: %w",
			pkg, err,
		)
	}
	manifest, err := VerifyArtifact(directory, identity, inputs)
	if err != nil {
		return Artifact{}, fmt.Errorf("verify prebuilt %s: %w", pkg, err)
	}
	return Artifact{Directory: directory, Manifest: manifest}, nil
}
