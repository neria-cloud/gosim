// gs01-consumer prepares and verifies disposable, pinned consumer evidence.
package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

const (
	consumerModule = "github.com/asalimonov/gquorum"
	gosimModule    = "github.com/neria-cloud/gosim"
	diskPackage    = "internal/recovery/disk"
	bootstrapName  = diskPackage + "/gs01_bootstrap_test.go"
	bootstrap      = `//go:build sim

package disk

import "os"

func init() {
 if err := os.MkdirAll("/tmp", 0700); err != nil { panic(err) }
 if err := os.Setenv("TMPDIR", "/tmp"); err != nil { panic(err) }
 if err := os.Setenv("GOTMPDIR", "/tmp"); err != nil { panic(err) }
}
`
)

var gateA = []string{
	"TestApprovalPublicationFaultsPreserveSourceAndGuard",
	"TestPhasePublicationReopenAndConcurrentReaders",
	"TestCancellationHistoryAndMatchingPointerClearing",
	"TestResetJournalHasNoReplaySeedsAndRecordsExactJoin",
}

type (
	edit    struct{ Before, After string }
	command struct {
		Args     []string
		Dir, Log string
		Exit     int
	}
)

type (
	selection struct{ Files, Tests []string }
	manifest  struct {
		Schema                                                                                   int
		Repository, Revision, Source, GoVersion, Host, GosimSource, GosimRevision, GosimDiffHash string
		Native, Sim                                                                              string
		Original, Final, Bodies, Production                                                      map[string]string
		Constraints                                                                              map[string]edit
		Added, ModuleEdits                                                                       map[string]string
		Selections                                                                               map[string]selection
		Features, Seeds, GateA                                                                   []string
		DeferredCases                                                                            map[string][]string
		Outcomes                                                                                 map[string]string
		Commands                                                                                 []command
	}
)

func main() {
	source := flag.String("source", "", "local gquorum Git repository")
	revision := flag.String("revision", "", "full consumer commit")
	output := flag.String("output", "", "fresh output directory")
	gosim := flag.String("gosim-source", "", "Gosim checkout")
	action := flag.String("action", "prepare", "prepare, verify, or run-a")
	flag.Parse()
	var err error
	switch *action {
	case "prepare":
		err = prepare(*source, *revision, *output, *gosim)
	case "verify":
		err = verifyOutput(*output)
	case "run-a":
		err = runA(*output)
	default:
		err = fmt.Errorf("unknown action %q", *action)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %v: %w", args, err)
	}
	return b, nil
}

func save(output string, m *manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "manifest.json"), append(b, '\n'), 0o600)
}

func read(output string) (*manifest, error) {
	b, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Schema != 1 {
		return nil, fmt.Errorf("unsupported manifest schema %d", m.Schema)
	}
	return &m, nil
}

// Reject links and special entries, including link-based traversal. The pinned
// source has neither links nor submodules; a changed baseline must be reviewed.
func extract(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue // Git archive commit provenance; no filesystem entry.
		}
		name := strings.TrimSuffix(h.Name, "/")
		if name == "" || !filepath.IsLocal(name) || path.Clean(name) != name || strings.Contains(name, "\\") || seen[name] {
			return fmt.Errorf("unsafe or repeated archive path %q", h.Name)
		}
		seen[name] = true
		target := filepath.Join(dest, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(h.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, tr)
			closeErr := f.Close()
			if err := errors.Join(copyErr, closeErr); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry %q type %d", h.Name, h.Typeflag)
		}
	}
}

func substitute(e constraint.Expr) (constraint.Expr, bool) {
	switch e := e.(type) {
	case *constraint.TagExpr:
		if e.Tag == "functional" {
			return &constraint.TagExpr{Tag: "sim"}, true
		}
		return e, false
	case *constraint.NotExpr:
		x, changed := substitute(e.X)
		return &constraint.NotExpr{X: x}, changed
	case *constraint.AndExpr:
		x, a := substitute(e.X)
		y, b := substitute(e.Y)
		return &constraint.AndExpr{X: x, Y: y}, a || b
	case *constraint.OrExpr:
		x, a := substitute(e.X)
		y, b := substitute(e.Y)
		return &constraint.OrExpr{X: x, Y: y}, a || b
	default:
		panic("unrecognized build expression")
	}
}

func body(b []byte) ([]byte, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "fixture.go", b, parser.PackageClauseOnly)
	if err != nil {
		return nil, err
	}
	return b[fs.Position(f.Package).Offset:], nil
}

func rewrite(b []byte, test bool) ([]byte, *edit, error) {
	content, err := body(b)
	if err != nil {
		return nil, nil, err
	}
	header := b[:len(b)-len(content)]
	lines := strings.Split(string(header), "\n")
	var expressions []constraint.Expr
	var modern constraint.Expr
	for _, line := range lines {
		if !constraint.IsGoBuild(line) && !constraint.IsPlusBuild(line) {
			continue
		}
		e, err := constraint.Parse(line)
		if err != nil {
			return nil, nil, err
		}
		if constraint.IsGoBuild(line) {
			if modern != nil {
				return nil, nil, errors.New("multiple go:build lines")
			}
			modern = e
		} else {
			expressions = append(expressions, e)
		}
	}
	var legacy constraint.Expr
	for _, e := range expressions {
		if legacy == nil {
			legacy = e
		} else {
			legacy = &constraint.AndExpr{X: legacy, Y: e}
		}
	}
	if modern != nil && legacy != nil && !equivalent(modern, legacy) {
		return nil, nil, errors.New("inconsistent modern/legacy constraints")
	}
	original := modern
	if original == nil {
		original = legacy
	}
	if original == nil {
		return b, nil, nil
	}
	updated, changed := substitute(original)
	if !changed {
		return b, nil, nil
	}
	if !test {
		return nil, nil, errors.New("non-test source uses functional build atom")
	}
	var out []string
	if modern != nil {
		out = append(out, "//go:build "+updated.String())
	}
	if legacy != nil {
		plus, err := constraint.PlusBuildLines(updated)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, plus...)
	}
	inserted := false
	var headerOut []string
	for _, line := range lines {
		if constraint.IsGoBuild(line) || constraint.IsPlusBuild(line) {
			if !inserted {
				headerOut = append(headerOut, out...)
				inserted = true
			}
			continue
		}
		headerOut = append(headerOut, line)
	}
	result := append([]byte(strings.Join(headerOut, "\n")), content...)
	return result, &edit{Before: original.String(), After: updated.String()}, nil
}

func equivalent(a, b constraint.Expr) bool {
	tags := map[string]bool{}
	a.Eval(func(s string) bool { tags[s] = true; return false })
	b.Eval(func(s string) bool { tags[s] = true; return false })
	names := make([]string, 0, len(tags))
	for s := range tags {
		names = append(names, s)
	}
	sort.Strings(names)
	if len(names) > 16 {
		return false
	}
	for mask := 0; mask < 1<<len(names); mask++ {
		values := map[string]bool{}
		for i, s := range names {
			values[s] = mask&(1<<i) != 0
		}
		eval := func(s string) bool { return values[s] }
		if a.Eval(eval) != b.Eval(eval) {
			return false
		}
	}
	return true
}

func files(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("non-regular source %s", p)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = digest(b)
		return nil
	})
	return result, err
}

func inventory(root, goos, tag string) (selection, error) {
	ctx := build.Default
	ctx.GOOS = goos
	ctx.GOARCH = "arm64"
	ctx.CgoEnabled = false
	ctx.BuildTags = []string{tag, "linkname"}
	dir := filepath.Join(root, diskPackage)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return selection{}, err
	}
	result := selection{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || name == filepath.Base(bootstrapName) {
			continue
		}
		match, err := ctx.MatchFile(dir, name)
		if err != nil {
			return result, err
		}
		if !match {
			continue
		}
		result.Files = append(result.Files, name)
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, 0)
		if err != nil {
			return result, err
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && strings.HasPrefix(fn.Name.Name, "Test") && fn.Recv == nil {
				result.Tests = append(result.Tests, fn.Name.Name)
			}
		}
	}
	sort.Strings(result.Tests)
	return result, nil
}

func mergeModule(sim, gosim string) error {
	original, err := os.ReadFile(filepath.Join(sim, "go.mod"))
	if err != nil {
		return err
	}
	f, err := modfile.Parse("go.mod", original, nil)
	if err != nil {
		return err
	}
	framework, err := os.ReadFile(filepath.Join(gosim, "go.mod"))
	if err != nil {
		return err
	}
	g, err := modfile.Parse("gosim/go.mod", framework, nil)
	if err != nil {
		return err
	}
	if g.Module.Mod.Path != gosimModule {
		return errors.New("wrong Gosim module")
	}
	versions := map[string]string{}
	for _, r := range f.Require {
		versions[r.Mod.Path] = r.Mod.Version
	}
	for _, r := range g.Require {
		if old := versions[r.Mod.Path]; old == "" || semver.Compare(old, r.Mod.Version) < 0 {
			if err := f.AddRequire(r.Mod.Path, r.Mod.Version); err != nil {
				return err
			}
		}
	}
	if err := f.AddRequire(gosimModule, "v0.0.0"); err != nil {
		return err
	}
	if err := f.AddReplace(gosimModule, "", gosim, ""); err != nil {
		return err
	}
	formatted, err := f.Format()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sim, "go.mod"), formatted, 0o600)
}

func recordCommand(output string, m *manifest, dir string, args ...string) error {
	log := filepath.Join(output, fmt.Sprintf("command-%02d.log", len(m.Commands)+1))
	f, err := os.Create(log)
	if err != nil {
		return err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdout = f
	cmd.Stderr = f
	runErr := cmd.Run()
	closeErr := f.Close()
	exit := 0
	if runErr != nil {
		exit = -1
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exit = ee.ExitCode()
		}
	}
	m.Commands = append(m.Commands, command{Args: args, Dir: dir, Log: log, Exit: exit})
	if err := save(output, m); err != nil {
		return err
	}
	if runErr != nil {
		return fmt.Errorf("command failed; see %s: %w", log, runErr)
	}
	return closeErr
}

func prepare(source, revision, output, gosim string) error {
	if source == "" || output == "" || gosim == "" || len(revision) != 40 {
		return errors.New("source, full revision, output and gosim-source are required")
	}
	var err error
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	gosim, err = filepath.Abs(gosim)
	if err != nil {
		return err
	}
	resolved, err := git(source, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(resolved)) != revision {
		return errors.New("revision must be full exact commit")
	}
	module, err := git(source, "show", revision+":go.mod")
	if err != nil {
		return err
	}
	mod, err := modfile.Parse("go.mod", module, nil)
	if err != nil {
		return err
	}
	if mod.Module == nil || mod.Module.Mod.Path != consumerModule {
		return errors.New("wrong consumer module identity")
	}
	tree, err := git(source, "ls-tree", "-r", revision)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(tree), "\n") {
		if strings.HasPrefix(line, "160000 ") {
			return errors.New("submodule baseline requires review")
		}
	}
	// Mkdir, rather than MkdirAll on the destination, refuses existing output.
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(output, 0o700); err != nil {
		return err
	}
	m := &manifest{Schema: 1, Repository: consumerModule, Revision: revision, Source: source, GoVersion: runtime.Version(), Host: runtime.GOOS + "/" + runtime.GOARCH, GosimSource: gosim, Native: filepath.Join(output, "native"), Sim: filepath.Join(output, "sim"), Bodies: map[string]string{}, Production: map[string]string{}, Constraints: map[string]edit{}, Added: map[string]string{}, ModuleEdits: map[string]string{}, Selections: map[string]selection{}, Features: []string{"rt", "fs"}, Seeds: []string{"1", "2", "3"}, GateA: gateA, Outcomes: map[string]string{"A": "not run", "B": "blocked in simulation: GS-03 increment 2 statx; native pending", "C/hardlink": "blocked in simulation: GS-03 increment 1 Linkat; native pending", "locks": "not qualified: existing lock stub; GS-02 required"}}
	m.DeferredCases = map[string][]string{
		"B": {"TestArchiveMoveFaultsPreserveEvidenceAndRequireJoinedSync", "TestArchiveMoveNeverReplacesExistingDestination"},
		"C": {"TestApprovalStagingDoesNotHideForeignContents/foreign_file", "TestApprovalStagingDoesNotHideForeignContents/nested_directory", "TestApprovalStagingDoesNotHideForeignContents/symlink", "TestApprovalStagingDoesNotHideForeignContents/hardlink"},
	}
	head, err := git(gosim, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	m.GosimRevision = strings.TrimSpace(string(head))
	diff, err := git(gosim, "diff", "HEAD", "--")
	if err != nil {
		return err
	}
	m.GosimDiffHash = digest(diff)
	archive, err := git(source, "archive", "--format=tar", revision)
	if err != nil {
		return err
	}
	for _, dest := range []string{m.Native, m.Sim} {
		if err := os.Mkdir(dest, 0o700); err != nil {
			return err
		}
		if err := extract(bytes.NewReader(archive), dest); err != nil {
			return err
		}
	}
	m.Original, err = files(m.Native)
	if err != nil {
		return err
	}
	for name := range m.Original {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		file := filepath.Join(m.Sim, filepath.FromSlash(name))
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		test := strings.HasSuffix(name, "_test.go")
		updated, change, err := rewrite(b, test)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if test {
			content, err := body(b)
			if err != nil {
				return err
			}
			m.Bodies[name] = digest(content)
		} else {
			m.Production[name] = digest(b)
		}
		if change != nil {
			m.Constraints[name] = *change
			if err := os.WriteFile(file, updated, 0o600); err != nil {
				return err
			}
		}
	}
	for _, item := range []struct{ name, goos, tag, root string }{{"native-linux", "linux", "functional", m.Native}, {"native-darwin", "darwin", "functional", m.Native}, {"sim-linux", "linux", "sim", m.Sim}} {
		selected, err := inventory(item.root, item.goos, item.tag)
		if err != nil {
			return err
		}
		m.Selections[item.name] = selected
	}
	if !reflect.DeepEqual(m.Selections["native-linux"], m.Selections["sim-linux"]) {
		return errors.New("transformed Linux selection differs")
	}
	for _, name := range gateA {
		found := false
		for _, selected := range m.Selections["sim-linux"].Tests {
			if selected == name {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("required test missing: %s", name)
		}
	}
	for name, content := range map[string]string{bootstrapName: bootstrap, "gosim.json": "{\"schema\":2,\"features\":[\"rt\",\"fs\"],\"simulation_os\":\"linux\"}\n"} {
		if _, exists := m.Original[name]; exists {
			return fmt.Errorf("bootstrap collision: %s", name)
		}
		if err := os.WriteFile(filepath.Join(m.Sim, name), []byte(content), 0o600); err != nil {
			return err
		}
		m.Added[name] = content
	}
	if err := mergeModule(m.Sim, gosim); err != nil {
		return err
	}
	if err := recordCommand(output, m, m.Sim, "go", "list", "-mod=mod", "-tags=linkname", "-deps", gosimModule+"/cmd/gosim-worker"); err != nil {
		return err
	}
	// Translation resolves the full application graph, beyond the worker imports.
	if err := recordCommand(output, m, m.Sim, "go", "mod", "download", "all"); err != nil {
		return err
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		b, err := os.ReadFile(filepath.Join(m.Sim, name))
		if err != nil {
			return err
		}
		m.ModuleEdits[name] = string(b)
	}
	m.Final, err = files(m.Sim)
	if err != nil {
		return err
	}
	if err := save(output, m); err != nil {
		return err
	}
	return verifyOutput(output)
}

func verifyOutput(output string) error {
	m, err := read(output)
	if err != nil {
		return err
	}
	for _, tree := range []struct {
		root   string
		hashes map[string]string
	}{{m.Native, m.Original}, {m.Sim, m.Final}} {
		if err := filepath.WalkDir(tree.root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(tree.root, p)
			if err != nil {
				return err
			}
			if rel == ".gosim" && d.IsDir() {
				return filepath.SkipDir
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("unexpected non-regular file: %s", p)
			}
			if _, ok := tree.hashes[filepath.ToSlash(rel)]; !ok {
				return fmt.Errorf("unexpected source file: %s", p)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	for name, want := range m.Original {
		b, err := os.ReadFile(filepath.Join(m.Native, name))
		if err != nil {
			return err
		}
		if digest(b) != want {
			return fmt.Errorf("native source changed: %s", name)
		}
	}
	for name, want := range m.Final {
		b, err := os.ReadFile(filepath.Join(m.Sim, name))
		if err != nil {
			return err
		}
		if digest(b) != want {
			return fmt.Errorf("sim source changed: %s", name)
		}
	}
	for name, want := range m.Production {
		b, err := os.ReadFile(filepath.Join(m.Sim, name))
		if err != nil {
			return err
		}
		if digest(b) != want {
			return fmt.Errorf("production changed: %s", name)
		}
	}
	for name, want := range m.Bodies {
		b, err := os.ReadFile(filepath.Join(m.Sim, name))
		if err != nil {
			return err
		}
		content, err := body(b)
		if err != nil {
			return err
		}
		if digest(content) != want {
			return fmt.Errorf("test body changed: %s", name)
		}
	}
	// Verify module preparation never lowers the consumer's requirements.
	original, err := os.ReadFile(filepath.Join(m.Native, "go.mod"))
	if err != nil {
		return err
	}
	final, err := os.ReadFile(filepath.Join(m.Sim, "go.mod"))
	if err != nil {
		return err
	}
	a, err := modfile.Parse("native/go.mod", original, nil)
	if err != nil {
		return err
	}
	b, err := modfile.Parse("sim/go.mod", final, nil)
	if err != nil {
		return err
	}
	if a.Go == nil || b.Go == nil || a.Go.Version != b.Go.Version {
		return errors.New("consumer Go directive changed")
	}
	versions := map[string]string{}
	for _, r := range b.Require {
		versions[r.Mod.Path] = r.Mod.Version
	}
	for _, r := range a.Require {
		if semver.Compare(versions[r.Mod.Path], r.Mod.Version) < 0 {
			return fmt.Errorf("consumer requirement lowered: %s", r.Mod.Path)
		}
	}
	for _, item := range []struct{ key, root, goos, tag string }{{"native-linux", m.Native, "linux", "functional"}, {"native-darwin", m.Native, "darwin", "functional"}, {"sim-linux", m.Sim, "linux", "sim"}} {
		got, err := inventory(item.root, item.goos, item.tag)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(got, m.Selections[item.key]) {
			return fmt.Errorf("source selection changed: %s", item.key)
		}
	}
	return nil
}

func runA(output string) error {
	if err := verifyOutput(output); err != nil {
		return err
	}
	m, err := read(output)
	if err != nil {
		return err
	}
	if m.GoVersion != runtime.Version() {
		return fmt.Errorf("compiler changed: prepared %s, running %s", m.GoVersion, runtime.Version())
	}
	launcher := filepath.Join(m.GosimSource, ".gosim/gosimtool")
	if err := recordCommand(output, m, m.Sim, launcher, "resolve-build", "-features", "rt,fs"); err != nil {
		return err
	}
	runErr := recordCommand(output, m, m.Sim, launcher, "test", "-features", "rt,fs", "-seeds", "1-3", "-run", "^("+strings.Join(gateA, "|")+")$", "./"+diskPackage)
	log, err := os.ReadFile(m.Commands[len(m.Commands)-1].Log)
	if err != nil {
		return err
	}
	m.Outcomes["A"] = "failed"
	if runErr == nil {
		runErr = validateAcceptanceLog(log, m.Seeds)
	}
	if runErr != nil && !bytes.Contains(log, []byte("=== RUN   ")) {
		m.Outcomes["A"] = "blocked before test execution: see " + m.Commands[len(m.Commands)-1].Log
	}

	verifyErr := verifyOutput(output)
	if runErr == nil && verifyErr == nil {
		m.Outcomes["A"] = "passed: four unchanged tests, seeds 1-3; no lock-exclusion or crash-enumeration claim"
	}
	if err := save(output, m); err != nil {
		return err
	}
	return errors.Join(runErr, verifyErr)
}

func validateAcceptanceLog(log []byte, seeds []string) error {
	for _, name := range gateA {
		for _, seed := range seeds {
			if bytes.Count(log, []byte("=== RUN   "+name+" (seed "+seed+")")) != 1 {
				return fmt.Errorf("expected one execution: %s seed %s", name, seed)
			}
		}
		if bytes.Count(log, []byte("--- PASS: "+name+" (")) != len(seeds) {
			return fmt.Errorf("missing passes: %s", name)
		}
	}
	if bytes.Contains(log, []byte("--- SKIP:")) || bytes.Contains(log, []byte("--- FAIL:")) {
		return errors.New("consumer run contains skipped or failed cases")
	}
	return nil
}
