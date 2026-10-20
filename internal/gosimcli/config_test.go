package gosimcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestCommandDoesNotRequireCertification(t *testing.T) {
	config := buildspec.Config{Schema: 2, Features: []string{"rt"}, SimulationOS: "linux"}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gosim.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	normal, err := resolveBuild("test", BuildFlags{ConfigPath: path})
	if err != nil {
		t.Fatal("ordinary command blocked without current certification", err)
	}
	candidate, err := resolveBuild("internal-candidate", BuildFlags{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := buildspec.Digest(normal.Identity)
	b, _ := buildspec.Digest(candidate.Identity)
	if a != b {
		t.Fatal("internal helper uses a different build identity/support policy")
	}
}
