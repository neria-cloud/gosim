package buildcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func distributionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"bin/go": "compiler", "pkg/tool/darwin_arm64/compile": "compile tool",
		"src/runtime/runtime.go": "package runtime\n", "src/go.mod": "module std\n",
		"VERSION": "go1.25.1\n", "go.env": "GOTOOLCHAIN=auto\n",
	} {
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDistributionFingerprintTracksInstalledContent(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"source", "compiler", "extra", "missing", "symlink"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			root := distributionFixture(t)
			before, err := fingerprintInstalled(root)
			if err != nil || !buildspec.ValidDigest(before.ToolchainDigest) ||
				!buildspec.ValidDigest(before.StdlibDigest) {
				t.Fatal(before, err)
			}
			source := filepath.Join(root, "src/runtime/runtime.go")
			switch mutation {
			case "source", "compiler":
				file := source
				if mutation == "compiler" {
					file = filepath.Join(root, "pkg/tool/darwin_arm64/compile")
				}
				stat, err := os.Stat(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(file, stat.ModTime(), stat.ModTime()); err != nil {
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
			}
			after, err := fingerprintInstalled(root)
			if mutation == "missing" || mutation == "symlink" {
				if err == nil {
					t.Fatal("accepted invalid installation")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if before == after {
				t.Fatal("input content change not fingerprinted")
			}
			if mutation == "compiler" && before.StdlibDigest != after.StdlibDigest {
				t.Fatal("compiler bytes changed source inventory")
			}
			if mutation != "compiler" && before.ToolchainDigest != after.ToolchainDigest {
				t.Fatal("source bytes changed tool inventory")
			}
		})
	}
}

func TestDistributionFingerprintIsIndependentOfLocation(t *testing.T) {
	t.Parallel()
	a, err := fingerprintInstalled(distributionFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := fingerprintInstalled(distributionFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("identical installations have different identities", a, b)
	}
}
