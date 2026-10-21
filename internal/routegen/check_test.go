package routegen

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckProfileDoesNotRepairStaleFiles(t *testing.T) {
	t.Parallel()
	tc, profile, options := currentGeneration(t)
	out := t.TempDir()
	if err := GenerateProfile(tc, profile.ID, []string{"arm64"}, out, options); err != nil {
		t.Fatal(err)
	}
	expected := t.TempDir()
	if err := GenerateProfile(tc, profile.ID, []string{"arm64"}, expected, options); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(out, ".gosim")); err != nil {
		t.Fatal(err)
	}
	check := func() error {
		if options.RefreshOrigin {
			return compareGeneratedOutputs(readOutputTree(t, expected), out)
		}
		return CheckProfile(tc, profile.ID, []string{"arm64"}, out)
	}
	if options.RefreshOrigin {
		if err := CheckProfile(tc, profile.ID, []string{"arm64"}, out); err == nil {
			t.Fatal("baseline check accepted another generation compiler")
		}
	}
	file := filepath.Join(out, "internal/kernel/routes_gen.go")
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := check(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("check rewrote generated file")
	}
	if err := os.WriteFile(file, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := readOutputTree(t, out)
	if err := check(); err == nil || !strings.Contains(err.Error(), "stale generated") {
		t.Fatal("stale output accepted", err)
	}
	if !reflect.DeepEqual(stale, readOutputTree(t, out)) {
		t.Fatal("check repaired stale output")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := check(); err == nil {
		t.Fatal("missing output accepted")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("check created missing output")
	}
	if err := CheckProfile(tc, profile.ID, []string{"arm64"}, ""); err == nil {
		t.Fatal("implicit output accepted")
	}
}
