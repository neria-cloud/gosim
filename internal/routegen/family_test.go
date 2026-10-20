package routegen

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/catalog"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestCurrentFamilyOriginRefresh(t *testing.T) {
	t.Parallel()
	r, err := buildcfg.ResolveRunningProcess(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	v, err := buildspec.ParseGoVersion(r.Toolchain.Version)
	if err != nil {
		t.Fatal(err)
	}
	p, err := catalog.Descriptor(v.Family(), r.Toolchain.HostArch)
	if err != nil {
		t.Fatal(err)
	}
	before, err := p.ContractDigest()
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := GenerateProfile(r.Toolchain, p.ID, []string{r.Toolchain.HostArch}, out, GenerateOptions{RefreshOrigin: true}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(out, "internal/compat", p.ID, "profile")
	data, err := os.ReadFile(filepath.Join(root, "origin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var origin spec.Origin
	if err := json.Unmarshal(data, &origin); err != nil {
		t.Fatal(err)
	}
	if origin.Schema != 2 || origin.BaselineToolchain != r.Toolchain.Version || origin.Family != v.Family() {
		t.Fatal("incorrect refreshed provenance", origin)
	}
	data, err = os.ReadFile(filepath.Join(root, "routes_"+r.Toolchain.HostArch+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record GenerationRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Schema != 2 || record.GeneratedWith != r.Toolchain.Version || record.Family != v.Family() {
		t.Fatal("incorrect generation record", record)
	}
	if _, err := os.Stat(filepath.Join(root, "contracts_"+r.Toolchain.HostArch+".json")); !os.IsNotExist(err) {
		t.Fatal("refresh wrote contract expectations", err)
	}
	after, err := p.ContractDigest()
	if err != nil || before != after {
		t.Fatal("refresh changed reviewed contracts", err)
	}
}
