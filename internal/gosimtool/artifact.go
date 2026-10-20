package gosimtool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// mkdirArtifactParents creates only real directories below a caller-owned root.
// Build locks and staging cannot be redirected through cached symlinks.
func mkdirArtifactParents(root, relative string) error {
	if relative == "." {
		return nil
	}
	if err := ValidateArtifactPath(filepath.ToSlash(relative)); err != nil {
		return err
	}
	current := root
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("project root is not a directory")
	}
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("artifact parent is not a real directory: %s", current)
		}
	}
	return nil
}

// MaterializeFiles writes translated sources into private staging. Validate the
// complete name set before the first write, then use exclusive, no-follow files.
func MaterializeFiles(directory string, files map[string][]byte) error {
	if err := ValidateGeneratedFiles(files); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("materialization root is not a real directory")
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := mkdirArtifactParents(directory, filepath.Dir(filepath.FromSlash(name))); err != nil {
			return err
		}
		f, err := os.OpenFile(
			filepath.Join(directory, filepath.FromSlash(name)),
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0o644,
		)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(files[name])
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return err
		}
	}
	return nil
}

// BuildArtifact holds an OS lock for this identity and package set, so concurrent
// processes reuse one complete build. Different packages never share a slot.
// The builder owns a private directory and returns its complete manifest; it
// must include its binary and every generated file. Only verified output moves
// into the final namespace. Task 21 connects the translator/compiler to this API.
func BuildArtifact(
	ctx context.Context,
	project string,
	identity buildspec.Identity,
	packages []PackageDigest,
	builder func(string) (Manifest, error),
) (artifact Artifact, err error) {
	identity, err = buildspec.NormalizeIdentity(identity)
	if err != nil {
		return artifact, err
	}
	packages, err = normalizePackages(packages)
	if err != nil {
		return artifact, err
	}
	directory, err := ArtifactDirectory(project, identity, packages)
	if err != nil {
		return artifact, err
	}
	relative, err := filepath.Rel(project, filepath.Dir(directory))
	if err != nil {
		return artifact, err
	}
	if err := mkdirArtifactParents(project, relative); err != nil {
		return artifact, err
	}
	unlock, err := buildspec.LockCache(ctx, directory+".lock")
	if err != nil {
		return artifact, fmt.Errorf("lock artifact: %w", err)
	}
	defer func() { err = errors.Join(err, unlock()) }()
	previous, verifyErr := VerifyArtifact(directory, identity, packages)
	if verifyErr == nil {
		return Artifact{directory, previous}, nil
	}
	var pathErr *os.PathError
	if errors.As(verifyErr, &pathErr) && !errors.Is(verifyErr, os.ErrNotExist) {
		return artifact, fmt.Errorf("read existing artifact: %w", verifyErr)
	}
	// A corrupt/incomplete old artifact is kept until replacement validates. A
	// complete valid artifact returned above is immutable and never overwritten.
	if builder == nil {
		return artifact, fmt.Errorf("no complete artifact and no builder: %w", verifyErr)
	}
	staging, err := os.MkdirTemp(filepath.Dir(directory), fmt.Sprintf(".build-%d-*", os.Getpid()))
	if err != nil {
		return artifact, err
	}
	defer os.RemoveAll(staging)
	m, err := builder(staging)
	if err != nil {
		return artifact, fmt.Errorf("build artifact: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return artifact, err
	}
	m, err = normalizeManifest(m)
	if err != nil {
		return artifact, err
	}
	if err := verifyManifestInputs(m, identity, packages); err != nil {
		return artifact, err
	}
	if err := verifyManifestFiles(staging, m); err != nil {
		return artifact, err
	}
	data, err := CanonicalManifest(m)
	if err != nil {
		return artifact, err
	}
	manifest, err := os.OpenFile(filepath.Join(staging, "manifest.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return artifact, err
	}
	_, writeErr := manifest.Write(data)
	if err := errors.Join(writeErr, manifest.Close()); err != nil {
		return artifact, err
	}
	var backup string
	if _, statErr := os.Lstat(directory); statErr == nil {
		backup, err = os.MkdirTemp(filepath.Dir(directory), ".replaced-*")
		if err != nil {
			return artifact, err
		}
		if err := os.Remove(backup); err != nil {
			return artifact, err
		}
		if err := os.Rename(directory, backup); err != nil {
			return artifact, err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return artifact, statErr
	}
	if err := os.Rename(staging, directory); err != nil {
		if backup != "" {
			err = errors.Join(err, os.Rename(backup, directory))
		}
		return artifact, fmt.Errorf("publish artifact: %w", err)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return artifact, fmt.Errorf("remove replaced artifact: %w", err)
		}
	}
	return Artifact{directory, m}, nil
}
