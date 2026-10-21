package audit

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/compat/spec"
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
