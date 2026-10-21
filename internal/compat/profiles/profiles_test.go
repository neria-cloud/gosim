package profiles

import (
	"os"
	"reflect"
	"testing"

	"github.com/neria-cloud/gosim/internal/testutil"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestDefinitionsOnlyEmbedAndIndependentViews(t *testing.T) {
	entries, err := metadata.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	defs, err := Definitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(defs) {
		t.Fatal("non-definition runtime embed")
	}
	for _, entry := range entries {
		data, err := metadata.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := spec.DecodeDefinition(data); err != nil {
			t.Fatalf("embedded %s: %v", entry.Name(), err)
		}
	}
	original := defs["1.25"].Delta.Declarations.Add[0].Hook.Symbol
	defs["1.25"].Delta.Declarations.Add[0].Hook.Symbol = "mutated"
	next, err := Definitions()
	if err != nil || next["1.25"].Delta.Declarations.Add[0].Hook.Symbol != original {
		t.Fatal("shared definition state", err)
	}
	for _, family := range buildspec.SupportedFamilies() {
		r, err := Resolve(family.Family)
		if err != nil {
			t.Fatal(err)
		}
		r.Declarations[0].Hook.Symbol = "mutated"
		again, err := Resolve(family.Family)
		if err != nil || again.Declarations[0].Hook.Symbol == "mutated" {
			t.Fatal("shared resolved state", err)
		}
		p, err := Descriptor(r.ProfileID, "arm64")
		if err != nil {
			t.Fatal(err)
		}
		p.Hooks[0].Symbol = "mutated"
		next, err := Descriptor(r.ProfileID, "arm64")
		if err != nil || next.Hooks[0].Symbol == "mutated" {
			t.Fatal("shared descriptor state", err)
		}
		policy, err := Policy(r.ProfileID)
		if err != nil {
			t.Fatal(err)
		}
		if err := policy.Validate(next); err != nil {
			t.Fatal(err)
		}
		policy.Modules[0].Path = "mutated"
		fresh, err := Policy(r.ProfileID)
		if err != nil || reflect.DeepEqual(policy, fresh) {
			t.Fatal("shared policy state", err)
		}
		shims, err := Shims(r.ProfileID, "amd64")
		if err != nil {
			t.Fatal(err)
		}
		shims[0].Symbol = "mutated"
		freshShims, err := Shims(r.ProfileID, "amd64")
		if err != nil || freshShims[0].Symbol == "mutated" {
			t.Fatal("shared shim state", err)
		}
	}
	if _, err := Descriptor(testutil.UnregisteredGo().Profile, "amd64"); err == nil {
		t.Fatal("unregistered descriptor accepted")
	}
	if _, err := Revision(testutil.UnregisteredGo().Profile); err == nil {
		t.Fatal("unregistered revision accepted")
	}
	if _, err := Shims("go125", "invalid"); err == nil {
		t.Fatal("invalid architecture accepted")
	}
}

func TestHistoricalPayloadPreserved(t *testing.T) {
	// Frozen after lossless comparison with both original family payloads.
	// No dependency on the retired trees or git history is retained.
	data, err := os.ReadFile("evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	if buildspec.Hash(data) != "a5eb57edd3aa48ed81ec3e120f9ba7c5db9cd28ae59e450ee12244719ea489d6" {
		t.Fatal("historical payload changed")
	}
}
