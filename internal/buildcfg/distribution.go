package buildcfg

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// Pinned from https://go.dev/dl/?mode=json&include=all on 2026-10-19.
// These are distribution trust anchors, not compatibility certification.
//
//go:embed distributions.json
var distributionJSON []byte

type archiveRecord struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
}

func distributionRecord(version, hostOS, arch string) (archiveRecord, error) {
	var records []archiveRecord
	if err := json.Unmarshal(distributionJSON, &records); err != nil {
		return archiveRecord{}, err
	}
	for _, r := range records {
		if r.Version == version && r.OS == hostOS && r.Arch == arch {
			return r, nil
		}
	}
	return archiveRecord{}, fmt.Errorf("no pinned official distribution for %s %s/%s", version, hostOS, arch)
}

func fetchArchive(record archiveRecord, dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	destination := filepath.Join(dir, record.SHA256+".tar.gz")
	if f, err := os.Open(destination); err == nil {
		// Verification always runs, including for cached archives.
		return f, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get("https://go.dev/dl/" + record.Filename)
	if err != nil {
		return nil, fmt.Errorf("download distribution: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", record.Filename, resp.Status)
	}
	f, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, record.Size+1))
	if err != nil {
		return nil, err
	}
	if n != record.Size || fmt.Sprintf("%x", h.Sum(nil)) != record.SHA256 {
		return nil, fmt.Errorf("official archive %s checksum/size mismatch", record.Filename)
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(f.Name(), destination); err != nil {
		return nil, err
	}
	return os.Open(destination)
}

func protectedDistributionPath(name string) bool {
	return name == "VERSION" || name == "go.env" || strings.HasPrefix(name, "src/") ||
		strings.HasPrefix(name, "bin/") || strings.HasPrefix(name, "lib/") || strings.HasPrefix(name, "pkg/tool/")
}

// verifyDistribution compares executable and source content, not mtimes or a
// self-reported go version. The only accepted installation transformation is
// the toolchain module's extra _go.mod copies, byte-identical to go.mod.
func verifyDistribution(root string, archive io.ReadSeeker, record archiveRecord) (Distribution, error) {
	h := sha256.New()
	n, err := io.Copy(h, archive)
	if err != nil {
		return Distribution{}, err
	}
	if n != record.Size || fmt.Sprintf("%x", h.Sum(nil)) != record.SHA256 {
		return Distribution{}, fmt.Errorf("cached distribution checksum mismatch: %s", record.Filename)
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return Distribution{}, err
	}
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return Distribution{}, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	expected := make(map[string]string)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Distribution{}, err
		}
		name := strings.TrimPrefix(header.Name, "go/")
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if name == header.Name || path.Clean(name) != name || !fs.ValidPath(name) {
			return Distribution{}, fmt.Errorf("invalid distribution path %q", header.Name)
		}
		if !protectedDistributionPath(name) {
			continue
		}
		if header.Typeflag != tar.TypeReg || expected[name] != "" {
			return Distribution{}, fmt.Errorf("invalid/duplicate distribution member %s", name)
		}
		h.Reset()
		if _, err := io.Copy(h, tr); err != nil {
			return Distribution{}, err
		}
		expected[name] = fmt.Sprintf("%x", h.Sum(nil))
	}
	if expected["bin/go"] == "" || expected["VERSION"] == "" || expected["src/runtime/runtime.go"] == "" {
		return Distribution{}, fmt.Errorf("incomplete official distribution")
	}
	seen := make(map[string]bool)
	err = filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if entry.IsDir() {
			if name == "." || name == "src" || name == "bin" || name == "lib" || name == "pkg" || protectedDistributionPath(name+"/") {
				return nil
			}
			return filepath.SkipDir
		}
		if !protectedDistributionPath(name) {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular GOROOT entry %s", name)
		}
		lookup := name
		if path.Base(name) == "_go.mod" {
			lookup = path.Join(path.Dir(name), "go.mod")
		}
		want, ok := expected[lookup]
		if !ok {
			return fmt.Errorf("unexpected GOROOT file %s", name)
		}
		got, err := hashFile(filename)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("modified GOROOT file %s", name)
		}
		seen[name] = true
		return nil
	})
	if err != nil {
		return Distribution{}, err
	}
	var sources []buildspec.Revision
	var names []string
	for name := range expected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !seen[name] {
			return Distribution{}, fmt.Errorf("missing GOROOT file %s", name)
		}
		if strings.HasPrefix(name, "src/") || name == "go.env" || name == "VERSION" {
			sources = append(sources, buildspec.Revision{Name: name, Digest: expected[name]})
		}
	}
	data, err := json.Marshal(sources)
	if err != nil {
		return Distribution{}, err
	}
	return Distribution{record.SHA256, buildspec.Hash(data), "https://go.dev/dl/" + record.Filename}, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
