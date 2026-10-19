package routegen

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildcfg"
)

func TestCheckProfileDoesNotRepairStaleFiles(t *testing.T) {
	t.Parallel()
	tc, err := buildcfg.Resolve("go1.26.9")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := GenerateProfile(tc, "go126", []string{"arm64"}, out, GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(out, "internal/kernel/routes_gen.go")
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckProfile(tc, "go126", []string{"arm64"}, out); err != nil {
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
	if err := CheckProfile(tc, "go126", []string{"arm64"}, out); err == nil || !strings.Contains(err.Error(), "stale generated") {
		t.Fatal("stale output accepted", err)
	}
	if !reflect.DeepEqual(stale, readOutputTree(t, out)) {
		t.Fatal("check repaired stale output")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := CheckProfile(tc, "go126", []string{"arm64"}, out); err == nil {
		t.Fatal("missing output accepted")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("check created missing output")
	}
	if err := CheckProfile(tc, "go126", []string{"arm64"}, ""); err == nil {
		t.Fatal("implicit output accepted")
	}
}
