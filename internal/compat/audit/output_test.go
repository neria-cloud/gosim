package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestExclusiveAuditPublication(t *testing.T) {
	report := ComparisonReport{Schema: 1, Complete: true, APIDiff: "skipped-no-parent", Findings: []Finding{{Kind: "signature", Severity: "incompatible", Key: []string{"p", "F"}, Message: "review implementation"}}}
	draft := DraftDelta{Schema: 1, Kind: "gosim-profile-draft", Complete: true}
	dir := filepath.Join(t.TempDir(), "comparison")
	if err := WriteComparison(dir, report, draft); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report.md", "report.json", "delta.draft.json"} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || len(data) == 0 {
			t.Fatal(name, err)
		}
	}
	if err := WriteComparison(dir, report, draft); err == nil {
		t.Fatal("overwrote directory")
	}
	partial := t.TempDir()
	sentinel := filepath.Join(partial, "report.md")
	if err := os.WriteFile(sentinel, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteComparison(partial, report, draft); err == nil {
		t.Fatal("reused partial directory")
	}
	if data, _ := os.ReadFile(sentinel); string(data) != "unrelated" {
		t.Fatal("overwrote unrelated file")
	}
	if _, err := os.Stat(filepath.Join(partial, "report.json")); !os.IsNotExist(err) {
		t.Fatal("published interrupted directory")
	}
	if err := publishExclusive(sentinel, []byte("replacement")); err == nil {
		t.Fatal("atomic publish clobbered existing path")
	}
	entries, err := os.ReadDir(partial)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary publication leak", entries, err)
	}
	report.Complete, draft.Complete = false, false
	if err := WriteComparison(filepath.Join(t.TempDir(), "incomplete"), report, draft); err != nil {
		t.Fatal(err)
	}
	snapshot := ParentObservation{Schema: 1, Complete: true, Observations: spec.ObservationSet{Contexts: []ContextResult{{Complete: true}}}}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := WriteObservation(path, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := WriteObservation(path, snapshot); err == nil {
		t.Fatal("overwrote snapshot")
	}
	snapshot.Observations.Contexts[0].Complete = false
	if err := WriteObservation(filepath.Join(t.TempDir(), "invalid.json"), snapshot); err == nil {
		t.Fatal("published incomplete snapshot")
	}
	if err := publishExclusive(filepath.Join(t.TempDir(), "absent", "report.json"), nil); err == nil {
		t.Fatal("ignored write failure")
	}
	report.Findings[0].After = []byte(`{`)
	if err := WriteComparison(filepath.Join(t.TempDir(), "invalid"), report, draft); err == nil {
		t.Fatal("published invalid JSON")
	}
	markdown, err := RenderReport(report)
	if err != nil || !strings.Contains(string(markdown), "API diff: skipped-no-parent") {
		t.Fatal(string(markdown), err)
	}
}
