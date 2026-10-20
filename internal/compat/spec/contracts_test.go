package spec

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestContractsDetectLayoutConstantAndSignatureDrift(t *testing.T) {
	t.Parallel()
	makeSources := func(code string) *sourceInspection {
		t.Helper()
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "test.go", code, 0)
		if err != nil {
			t.Fatal(err)
		}
		return &sourceInspection{ctx: t.Context(), fset: fset, syntax: map[string][]*ast.File{"fixture": {f}}, types: map[string]*types.Package{}, loading: map[string]bool{}, sizes: types.SizesFor("gc", runtime.GOARCH), version: runtime.Version()}
	}
	baseline := "package fixture\ntype Record struct { First uint32; Second uint64 }; const Limit = 42\n"
	sources := makeSources(baseline)
	pkg, err := sources.Import("fixture")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := describeLayout(pkg, sources.sizes, "Record")
	if err != nil {
		t.Fatal(err)
	}
	manifest := ContractManifest{Layouts: []LayoutContract{layout}, Constants: []ConstantContract{{Package: "fixture", Symbol: "Limit", Value: "42"}}}
	if err := sources.verifyLayoutsAndConstants(manifest); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct{ source, want string }{
		{strings.Replace(baseline, "First uint32", "First [2]uint32", 1), "layout ABI drift"},
		{strings.Replace(baseline, "First uint32", "First uint64", 1), "layout ABI drift"},
		{strings.Replace(baseline, "42", "43", 1), "constant ABI drift"},
		{strings.Replace(baseline, "Limit", "Other", 1), "required constant"},
		{strings.Replace(baseline, "Record", "Other", 1), "required type"},
	} {
		if err := makeSources(change.source).verifyLayoutsAndConstants(manifest); err == nil || !strings.Contains(err.Error(), change.want) {
			t.Fatalf("%s: %v", change.want, err)
		}
	}
	// Noncontract function bodies and comments are not baseline source-byte gates.
	if err := makeSources(baseline + "\n// changed implementation\nfunc unrelated() int { return 123 }\n").verifyLayoutsAndConstants(manifest); err != nil {
		t.Fatal(err)
	}
}

func testCompatibilityInputs() compatibilityInputs {
	d := strings.Repeat("a", 64)
	return compatibilityInputs{Schema: 1, Family: "1.26", Toolchain: "go1.26.9", ContractDigest: d, CheckerRevision: d, HostOS: runtime.GOOS, Architecture: runtime.GOARCH, Fingerprints: buildspec.Fingerprints{ToolchainDigest: d, StdlibDigest: d}}
}

func TestContractCacheSuccessCorruptionAndInputIsolation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inputs := testCompatibilityInputs()
	calls := 0
	verify := func() error { calls++; return nil }
	check := func(in compatibilityInputs) {
		t.Helper()
		if err := checkCachedContracts(t.Context(), root, in, verify); err != nil {
			t.Fatal(err)
		}
	}
	check(inputs)
	check(inputs)
	if calls != 1 {
		t.Fatal("cache did not reuse successful check", calls)
	}
	for _, change := range []func(*compatibilityInputs){
		func(in *compatibilityInputs) { in.Toolchain = "go1.26.1" },
		func(in *compatibilityInputs) { in.Fingerprints.StdlibDigest = strings.Repeat("b", 64) },
		func(in *compatibilityInputs) { in.Fingerprints.ToolchainDigest = strings.Repeat("b", 64) },
		func(in *compatibilityInputs) { in.ContractDigest = strings.Repeat("b", 64) },
		func(in *compatibilityInputs) { in.CheckerRevision = strings.Repeat("b", 64) },
		func(in *compatibilityInputs) { in.HostOS = "other" },
		func(in *compatibilityInputs) { in.Architecture = "other" },
		func(in *compatibilityInputs) { in.Race = true },
		func(in *compatibilityInputs) {
			in.Modules = []buildspec.Revision{{Name: "dep", Digest: strings.Repeat("b", 64)}}
		},
	} {
		changed := inputs
		change(&changed)
		before := calls
		check(changed)
		if calls != before+1 {
			t.Fatal("cache ignored input change")
		}
	}
	files, err := filepath.Glob(filepath.Join(root, "compat/v1/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte(`{"schema":999}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := calls
	check(inputs)
	if calls != before+1 {
		t.Fatal("corrupt success reused")
	}
}

func TestContractCacheConcurrentCancellationAndFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inputs := testCompatibilityInputs()
	expected := errors.New("drift")
	for range 2 {
		if err := checkCachedContracts(t.Context(), root, inputs, func() error { return expected }); !errors.Is(err, expected) {
			t.Fatal(err)
		}
	}
	files, _ := filepath.Glob(filepath.Join(root, "compat/v1/*.json"))
	if len(files) != 0 {
		t.Fatal("cached failed check")
	}
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	verify := func() error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	}
	done := make(chan error, 2)
	go func() { done <- checkCachedContracts(t.Context(), root, inputs, verify) }()
	<-started
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := checkCachedContracts(ctx, root, inputs, verify); !errors.Is(err, context.DeadlineExceeded) {
		close(release)
		t.Fatal(err)
	}
	go func() { done <- checkCachedContracts(t.Context(), root, inputs, verify) }()
	close(release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicated compatibility check", calls.Load())
	}
	ctx, cancel = context.WithCancel(t.Context())
	cancel()
	if err := checkCachedContracts(ctx, root, inputs, verify); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation on warm hit", err)
	}
}

func TestContractModuleContentIgnoresMtimeShortcuts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "dependency.go")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := contractModuleDigest(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after!"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := contractModuleDigest(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("ABI module mutation with preserved size/mtime reused digest")
	}
}

func TestSelectedSourcesDistinguishHostLinuxAndRace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.25.0\n")
	write("api.go", "package fixture\n")
	write("host_darwin.go", "package fixture\nfunc Host(int) {}\n")
	write("host_linux.go", "package fixture\nfunc Host(string) {}\n")
	write("race.go", "//go:build race\n\npackage fixture\nfunc RaceOnly() {}\n")
	tc := buildspec.Toolchain{Version: runtime.Version(), GOROOT: runtime.GOROOT(), Executable: filepath.Join(runtime.GOROOT(), "bin/go"), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	for _, test := range []struct {
		os, signature string
		race          bool
	}{{"darwin", "func(int)", false}, {"linux", "func(string)", false}, {"linux", "func(string)", true}} {
		source, err := inspectSources(t.Context(), tc, dir, SourceContext{OS: test.os, Arch: tc.HostArch, Race: test.race}, false, []string{"fixture"})
		if err != nil {
			t.Fatal(err)
		}
		if err := source.verifyHooks([]Hook{{Package: "fixture", Symbol: "Host", Signature: test.signature}}); err != nil {
			t.Fatal(err)
		}
		err = source.verifyHooks([]Hook{{Package: "fixture", Symbol: "RaceOnly", Signature: "func()"}})
		if (err == nil) != test.race {
			t.Fatalf("race selection %t: %v", test.race, err)
		}
	}
}
