package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/profiles"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/testutil"
)

func TestComparisonFindingsAndProposals(t *testing.T) {
	c := InspectedContext{OS: "linux", Arch: "arm64"}
	d := spec.Declaration{Hook: spec.Hook{Package: "p", Symbol: "F", Signature: "func()", Target: "implementation.F"}}
	parent := spec.ResolvedFamily{Family: "1.25", Declarations: []spec.Declaration{d}, Packages: []spec.PackagePolicy{{Path: "p", Mode: "translate"}}, Architectures: map[string]spec.ArchitectureRecords{"arm64": {Constants: []spec.ConstantContract{{Package: "p", Symbol: "C", Value: "1"}}}}}
	origin := spec.Origin{Files: []spec.SourceFile{{Path: "src/p/p.go", Digest: "old"}}}
	after := spec.ObservationSet{Contexts: []ContextResult{{Context: c, Complete: true}}, APIPackages: []string{"p"}}
	for _, r := range expectedContracts(parent, c) {
		after.Records = append(after.Records, r)
	}
	after.Records = append(after.Records, spec.Observation{Kind: "source", Key: []string{"src/p/p.go"}, Context: c, Present: true, Value: auditJSON("old")}, spec.Observation{Kind: "api", Key: []string{"p", "Old"}, Context: c, Present: true, Value: auditJSON("func()")})
	baseline := after
	baseline.Records = slices.Clone(after.Records)
	before := &ParentObservation{Observations: baseline}
	findings, proposals, err := CompareObservations(parent, origin, before, after)
	if err != nil || len(findings) != 0 || len(proposals) != 0 {
		t.Fatal(findings, proposals, err)
	}
	for i, r := range after.Records {
		switch r.Kind {
		case "declaration":
			after.Records[i].Value = auditJSON("func(int)")
		case "constant":
			after.Records[i].Value = auditJSON("2")
		case "source":
			after.Records[i].Value = auditJSON("new")
		case "api":
			after.Records[i].Key = []string{"p", "New"}
		}
	}
	findings, proposals, err = CompareObservations(parent, origin, before, after)
	if err != nil || len(proposals) != 2 {
		t.Fatal(findings, proposals, err)
	}
	kinds := map[string]bool{}
	for _, f := range findings {
		kinds[f.Kind] = true
	}
	for _, kind := range []string{"signature", "constant", "source", "api-added", "api-removed"} {
		if !kinds[kind] {
			t.Fatal("missing", kind)
		}
	}
	for _, p := range proposals {
		if !p.NeedsImplementation || p.Operation != "replace" {
			t.Fatal(p)
		}
	}
	draft := DraftDelta{Schema: 1, Kind: "gosim-profile-draft", Family: "1.26", Parent: "1.25", Complete: true, Proposals: proposals}
	if _, err := spec.DecodeDefinition(auditJSON(draft)); err == nil {
		t.Fatal("draft installed as definition")
	}
	original := string(auditJSON(findings))
	slices.Reverse(after.Records)
	again, _, err := CompareObservations(parent, origin, before, after)
	if err != nil || original != string(auditJSON(again)) {
		t.Fatal("nondeterministic findings", err)
	}
	without, _, err := CompareObservations(parent, origin, nil, after)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range without {
		if len(f.Kind) > 3 && f.Kind[:3] == "api" {
			t.Fatal("API flood without snapshot")
		}
	}
	// Different race-selected signatures cannot become a single metadata operation.
	race := c
	race.Race = true
	after.Contexts = append(after.Contexts, ContextResult{Context: race, Complete: true})
	for _, r := range baseline.Records {
		r.Context = race
		after.Records = append(after.Records, r)
	}
	_, proposals, err = CompareObservations(parent, origin, nil, after)
	if err != nil || len(proposals) != 0 {
		t.Fatal("ambiguous proposal", proposals, err)
	}
	after.Contexts[1].Complete = false
	after.Contexts[1].Errors = []string{"loader failed"}
	findings, proposals, err = CompareObservations(parent, origin, nil, after)
	if err == nil || len(proposals) != 0 {
		t.Fatal("incomplete scan proposed edits")
	}
	if !slices.ContainsFunc(findings, func(f Finding) bool { return f.Kind == "inspection-error" }) {
		t.Fatal(findings)
	}
	missing := baseline
	missing.Records = slices.Clone(baseline.Records)
	for i, r := range missing.Records {
		if r.Kind == "declaration" {
			missing.Records[i].Present = false
			missing.Records[i].Value = nil
		}
	}
	_, proposals, err = CompareObservations(parent, origin, nil, missing)
	if err != nil || len(proposals) != 1 || proposals[0].Operation != "remove" {
		t.Fatal(proposals, err)
	}
	stale := *before
	stale.Observations.APIPackages = []string{"wrong"}
	if _, _, err := CompareObservations(parent, origin, &stale, baseline); err == nil {
		t.Fatal("mismatched inventory accepted")
	}
}

func TestActualCandidateComparison(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	report, draft, err := Compare(ctx, CompareRequest{Directory: root, ParentProfile: "go125", Arches: []string{runtime.GOARCH}})
	if runtime.Version() == "go1.25.14" {
		if err == nil {
			t.Fatal("same-family candidate accepted")
		}
		return
	}
	if runtime.Version() != "go1.26.9" {
		t.Skip("recorded candidate fixture requires go1.26.9")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete || !draft.Complete || report.APIDiff != "skipped-no-parent" || report.ParentObservationUsed {
		t.Fatal(report.Complete, report.APIDiff)
	}
	if !slices.ContainsFunc(report.Findings, func(f Finding) bool { return f.Kind == "signature" }) {
		t.Fatal("known signature changes absent")
	}
	if !slices.ContainsFunc(report.Findings, func(f Finding) bool { return f.Kind == "source" && slices.Contains(f.Key, "src/crypto/rand/util.go") }) {
		t.Fatal("crypto source change absent")
	}
	data, _ := json.Marshal(draft)
	if _, err := spec.DecodeDefinition(data); err == nil {
		t.Fatal("installable draft")
	}
	t.Logf("%s complete candidate: %d findings, %d proposals", runtime.Version(), len(report.Findings), len(draft.Proposals))
}

func TestNoChangeFamilyOnboarding(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	root := testutil.CopySource(t, "../../..")
	future := testutil.UnregisteredGo()
	parentFamily := fmt.Sprintf("1.%d", future.Minor-1)
	parent, err := profiles.Resolve(parentFamily)
	if err != nil {
		t.Fatal(err)
	}
	implementation := func() map[string]string {
		files := map[string]string{}
		for _, tree := range []string{"host", "hooks", "shims"} {
			err := filepath.WalkDir(filepath.Join(root, "internal/compat", tree), func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || d.Name() == "unsupported.go" {
					return nil
				}
				data, err := os.ReadFile(path)
				if err == nil {
					files[path] = buildspec.Hash(data)
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return files
	}
	before := implementation()
	definition := spec.FamilyDefinition{Schema: 1, Family: future.Family, Parent: parentFamily, Supported: true}
	dir := filepath.Join(root, "internal/compat/profiles")
	if err := os.WriteFile(filepath.Join(dir, future.Profile+".json"), auditJSON(definition), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := profiles.GenerateMetadata(ctx, root, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, implementation()) {
		t.Fatal("no-change family copied or changed implementation files")
	}
	defs, err := spec.LoadDefinitions(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := spec.ResolveDefinitions(defs, future.Family)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Chain) != len(parent.Chain)+1 || !reflect.DeepEqual(resolved.Declarations, parent.Declarations) || !reflect.DeepEqual(resolved.Architectures, parent.Architectures) {
		t.Fatal("cumulative inheritance changed")
	}
	r, err := buildcfg.ResolveRunningProcess(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := buildspec.OutputCommand(ctx, r.Toolchain, root, "test", "-ldflags=-checklinkname=0", "-tags=linkname", "./internal/buildspec", "./internal/buildcfg", "./internal/compat/spec", "./internal/compat/profiles", "./internal/hostabi")
	// These native tests inspect both source architectures; do not inherit an
	// explicit host GOARCH into their per-context environment assertions.
	cmd.Env = slices.DeleteFunc(cmd.Env, func(entry string) bool {
		return strings.HasPrefix(entry, "GOARCH=") || strings.HasPrefix(entry, "GOOS=")
	})
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("no-change family focused acceptance: %v\n%s", err, out)
	} else {
		t.Logf("%s hypothetical %s admission tests: %s", runtime.Version(), future.Family, out)
	}
}

func TestCandidateMissingPackageAndIncompleteReport(t *testing.T) {
	if !strings.HasPrefix(runtime.Version(), "go1.26.") {
		t.Skip("actual next-minor fixture uses running Go 1.26")
	}
	t.Parallel()
	root := testutil.CopySource(t, "../../..")
	path := filepath.Join(root, "internal/compat/profiles/go125.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, broken := range []bool{false, true} {
		definition, err := spec.DecodeDefinition(original)
		if err != nil {
			t.Fatal(err)
		}
		pkg := "internal/synctestmoved"
		if broken {
			pkg = frameworkModule + "/internal/auditbroken"
			dir := filepath.Join(root, "internal/auditbroken")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package auditbroken\nfunc Broken(\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for i, d := range definition.Delta.Declarations.Add {
			if d.Hook.Package == "internal/synctest" {
				definition.Delta.Declarations.Add[i].Hook.Package = pkg
			}
		}
		if err := os.WriteFile(path, auditJSON(definition), 0o600); err != nil {
			t.Fatal(err)
		}
		report, draft, err := Compare(t.Context(), CompareRequest{Directory: root, ParentProfile: "go125", Arches: []string{runtime.GOARCH}})
		if broken {
			if err == nil || report.Complete || report.Schema != 1 {
				t.Fatal("parse failure lost incomplete report", err)
			}
		} else {
			if err != nil || !report.Complete {
				t.Fatal("missing package aborted comparison", err)
			}
			for _, kind := range []string{"package", "missing", "signature", "source"} {
				if !slices.ContainsFunc(report.Findings, func(f Finding) bool { return f.Kind == kind }) {
					t.Fatal("missing finding", kind)
				}
			}
		}
		dir := filepath.Join(t.TempDir(), "report")
		if err := WriteComparison(dir, report, draft); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "report.json")); err != nil {
			t.Fatal(err)
		}
	}
}
