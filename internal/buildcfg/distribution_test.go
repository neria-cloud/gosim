package buildcfg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func distributionFixture(t *testing.T) (string, []byte, archiveRecord) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"bin/go": "test executable", "pkg/tool/darwin_arm64/compile": "test compiler",
		"src/runtime/runtime.go": "package runtime\n", "src/go.mod": "module std\n",
		"VERSION": "go1.26.9\n", "go.env": "GOTOOLCHAIN=auto\n",
	}
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	var names []string
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		body := []byte(files[name])
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := tw.WriteHeader(&tar.Header{Name: "go/" + name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	data := archive.Bytes()
	return root, data, archiveRecord{Filename: "test.tar.gz", SHA256: buildspec.Hash(data), Size: int64(len(data))}
}

func TestDistributionRejectsChangedContent(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"source", "compiler", "extra", "missing", "symlink", "archive"} {
		t.Run(mutation, func(t *testing.T) {
			root, data, record := distributionFixture(t)
			before, err := verifyDistribution(root, bytes.NewReader(data), record)
			if err != nil || !buildspec.ValidDigest(before.SourceDigest) {
				t.Fatalf("original archive: %+v %v", before, err)
			}
			source := filepath.Join(root, "src/runtime/runtime.go")
			switch mutation {
			case "source", "compiler":
				file := source
				if mutation == "compiler" {
					file = filepath.Join(root, "pkg/tool/darwin_arm64/compile")
				}
				info, err := os.Stat(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte("modified"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "extra":
				if err := os.WriteFile(filepath.Join(root, "src/runtime/extra.go"), []byte("package runtime"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "missing", "symlink":
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				if mutation == "symlink" {
					if err := os.Symlink(filepath.Join(root, "VERSION"), source); err != nil {
						t.Fatal(err)
					}
				}
			case "archive":
				data[len(data)/2] ^= 1
			}
			if _, err := verifyDistribution(root, bytes.NewReader(data), record); err == nil {
				t.Fatal("accepted tampered distribution")
			}
		})
	}
}

func TestDistributionAllowsOnlyVerifiedModuleCopies(t *testing.T) {
	t.Parallel()
	root, data, record := distributionFixture(t)
	copy := filepath.Join(root, "src/_go.mod")
	if err := os.WriteFile(copy, []byte("module std\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyDistribution(root, bytes.NewReader(data), record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copy, []byte("module modified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyDistribution(root, bytes.NewReader(data), record); err == nil {
		t.Fatal("accepted changed module-copy content")
	}
}
