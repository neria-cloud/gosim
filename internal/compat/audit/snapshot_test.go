package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/testutil"
)

func TestLoadParentValidatesPoliciesWithoutSupportAdmission(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal/compat/profiles")
	if err := os.CopyFS(dir, os.DirFS("../profiles")); err != nil {
		t.Fatal(err)
	}
	parent, origin, err := loadParent(root, "go125")
	if err != nil {
		t.Fatal(err)
	}
	defs, err := spec.LoadDefinitions(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	baseline := defs[parent.Family]
	baseline.Supported = false
	write := func(name string, value any) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), auditJSON(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go125.json", baseline)
	if retired, _, err := loadParent(root, "go125"); err != nil || retired.Supported {
		t.Fatal("valid retired parent rejected", err)
	}
	future := testutil.UnregisteredGo()
	write(future.Profile+".json", spec.FamilyDefinition{Schema: 1, Family: future.Family, Parent: fmt.Sprintf("1.%d", future.Minor-1)})
	origin.Family, origin.BaselineToolchain = future.Family, fmt.Sprintf("go1.%d.0", future.Minor)
	write(future.Profile+".origin.json", origin)
	if _, _, err := loadParent(root, future.Profile); err != nil {
		t.Fatal("valid unregistered parent rejected", err)
	}
	baseline.Delta.Bridges.Add = slices.DeleteFunc(baseline.Delta.Bridges.Add, func(b spec.Bridge) bool {
		return b.Package == "crypto/subtle"
	})
	write("go125.json", baseline)
	for _, profile := range []string{"go125", "go126", future.Profile} {
		if _, _, err := loadParent(root, profile); err == nil || !strings.Contains(err.Error(), "bridge crypto/subtle lacks a boundary contract") {
			t.Fatalf("%s accepted invalid inherited policy: %v", profile, err)
		}
	}
}

func TestActualParentSnapshotAndValidation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	v, err := buildspec.ParseGoVersion(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	family, err := buildspec.LookupFamily(v)
	if err != nil {
		t.Fatal(err)
	}
	root := testutil.CopySource(t, "../../..")
	parent, origin, err := loadParent(root, family.Profile)
	if err != nil {
		t.Fatal(err)
	}
	req := ObserveRequest{Directory: root, Profile: family.Profile, Output: filepath.Join(root, "must-not-publish.json"), Arches: []string{runtime.GOARCH}}
	if runtime.Version() != origin.BaselineToolchain {
		if _, err := Observe(ctx, req); err == nil || !strings.Contains(err.Error(), "recorded compiler") {
			t.Fatal(err)
		}
		t.Skip("actual parent snapshot requires its recorded compiler; mismatch rejection verified")
	}
	start := time.Now()
	snapshot, err := Observe(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Complete || snapshot.Compiler.Version != runtime.Version() || len(snapshot.Observations.Records) == 0 {
		t.Fatal("incomplete snapshot")
	}
	if _, err := os.Stat(req.Output); !os.IsNotExist(err) {
		t.Fatal("Observe published output", err)
	}
	tc := buildspec.Toolchain{Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	contexts, err := observationContexts(tc, parent, req.Arches)
	if err != nil {
		t.Fatal(err)
	}
	validate := func(s ParentObservation) error {
		return validateParentObservation(s, parent, origin, snapshot.InspectorDigest, snapshot.ModuleInputs, contexts)
	}
	if err := validate(snapshot); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ParentObservation){
		"schema": func(s *ParentObservation) { s.Schema++ }, "incomplete": func(s *ParentObservation) { s.Complete = false },
		"family": func(s *ParentObservation) { s.Family = "1.99" }, "baseline": func(s *ParentObservation) { s.Compiler.Version = "go1.25.999" },
		"fingerprint":   func(s *ParentObservation) { s.Compiler.Fingerprints.StdlibDigest = "invalid" },
		"definition":    func(s *ParentObservation) { s.DefinitionDigest = strings.Repeat("a", 64) },
		"origin":        func(s *ParentObservation) { s.OriginDigest = strings.Repeat("b", 64) },
		"inspector":     func(s *ParentObservation) { s.InspectorDigest = strings.Repeat("c", 64) },
		"module":        func(s *ParentObservation) { s.ModuleInputs = nil },
		"api inventory": func(s *ParentObservation) { s.Observations.APIPackages = s.Observations.APIPackages[1:] },
		"contexts":      func(s *ParentObservation) { s.Observations.Contexts = s.Observations.Contexts[1:] },
		"context error": func(s *ParentObservation) { s.Observations.Contexts[0].Errors = []string{"failed"} },
		"records":       func(s *ParentObservation) { s.Observations.Records = nil },
		"duplicate": func(s *ParentObservation) {
			s.Observations.Records = append(s.Observations.Records, s.Observations.Records[0])
		},
		"invalid value": func(s *ParentObservation) {
			s.Observations.Records[0].Present = true
			s.Observations.Records[0].Value = []byte(`{`)
		},
		"missing declaration": func(s *ParentObservation) {
			for i, r := range s.Observations.Records {
				if r.Kind == "declaration" {
					s.Observations.Records = append(s.Observations.Records[:i], s.Observations.Records[i+1:]...)
					break
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			var changed ParentObservation
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			mutate(&changed)
			if err := validate(changed); err == nil {
				t.Fatal("accepted invalid snapshot")
			}
		})
	}
	// Target data alone cannot invalidate a captured parent.
	future := testutil.UnregisteredGo()
	definition := spec.FamilyDefinition{Schema: 1, Family: future.Family, Parent: fmt.Sprintf("1.%d", future.Minor-1), Supported: true}
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/compat/profiles", future.Profile+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	fresh, _, err := loadParent(root, family.Profile)
	if err != nil {
		t.Fatal(err)
	}
	inspector, err := inspectorDigest(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateParentObservation(snapshot, fresh, origin, inspector, snapshot.ModuleInputs, contexts); err != nil {
		t.Fatal("target data staled snapshot", err)
	}
	changedParent := parent
	changedParent.DefinitionDigest = strings.Repeat("f", 64)
	if err := validateParentObservation(snapshot, changedParent, origin, inspector, snapshot.ModuleInputs, contexts); err == nil {
		t.Fatal("parent edit accepted")
	}
	changedOrigin := origin
	changedOrigin.Files = append([]spec.SourceFile(nil), origin.Files...)
	changedOrigin.Files[0].Digest = strings.Repeat("e", 64)
	if err := validateParentObservation(snapshot, parent, changedOrigin, inspector, snapshot.ModuleInputs, contexts); err == nil {
		t.Fatal("origin edit accepted")
	}
	t.Logf("actual %s parent snapshot: %d contexts, %d API packages, %d records, %s", runtime.Version(), len(snapshot.Observations.Contexts), len(snapshot.Observations.APIPackages), len(snapshot.Observations.Records), time.Since(start))
}

func TestSnapshotContextRequests(t *testing.T) {
	tc := buildspec.Toolchain{HostOS: "darwin", HostArch: "arm64"}
	parent := spec.ResolvedFamily{Declarations: []spec.Declaration{{Hook: spec.Hook{Context: "host"}}}}
	contexts, err := observationContexts(tc, parent, nil)
	if err != nil || len(contexts) != 6 {
		t.Fatal(contexts, err)
	}
	for _, arches := range [][]string{{"mips"}, {"arm64", "arm64"}, {""}} {
		if _, err := observationContexts(tc, parent, arches); err == nil {
			t.Fatal("invalid architecture accepted")
		}
	}
}
