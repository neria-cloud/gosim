package candidate

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/hostabi"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

// Test-only source injection exercises private overflow and cleanup paths after
// translation without exposing application hooks or a public fault route. The
// copied framework has its own content identity and candidate artifact; the
// production checkout and its artifact manifests are never patched.
func checkTranslatedFaults(t *testing.T, cfg buildspec.Config) {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".gosim" || rel == "docs" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(project, rel), 0o755)
		}
		if rel == ".git" || filepath.Ext(rel) == ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(project, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"lifecycle_fakes", "io_fakes"} {
		data, err := os.ReadFile(filepath.Join(root, "internal/kernel", source+"_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, "internal/kernel", source+"_fixture.go"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "internal/tests/features/testdata/fault_kernel.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "internal/kernel/fault_fixture.go"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "internal/kernel/dispatcher.go")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	needle := "k := d.kernel"
	if bytes.Count(data, []byte(needle)) != 1 {
		t.Fatal("fault injection point changed")
	}
	data = bytes.Replace(data, []byte(needle), []byte(`if inv.Route == RouteID(0xfffffff0) { d.kernel.translatedFault(inv.Args[0].(string)); _ = d.Complete(inv.Context.InvocationID, Result{}); return }; `+needle), 1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(project, "internal/tests/faultfixture")
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "fault_test.go"), []byte(faultApplication), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(root)
	built, err := Build(cfg, buildspec.BuildOptions{Race: hostabi.RaceEnabled}, []string{"./internal/tests/faultfixture"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, want string }{
		{"open-fs", ""},
		{"open-net", ""},
		{"controls-fs", ""},
		{"controls-net", ""},
		{"open-close", ""},
		{"prepare-fs", ""},
		{"prepare-net", ""},
		{"bindings-fs", ""},
		{"bindings-net", ""},
		{"runtime-prepare", ""},
		{"restart", ""},
		{"publish-fs", "publication"},
		{"publish-net", "publication"},
		{"publish-nil", "publication"},
		{"publish-yield", "publication"},
		{"quiesce", "quiesce failure"},
		{"settle", "settle fault"},
		{"release", "release fault"},
		{"detach", "detach failure"},
		{"close", "close failure"},
		{"destroy", "destroy failure"},
		{"cleanup-all", "settle fault"},
		{"rollback-errors", "release fault"},
		{"machine-overflow", "exhausted"},
		{"epoch-overflow", "exhausted"},
		{"wait-overflow", "exhausted"},
		{"fd-overflow", ""},
		{"invocation-overflow", ""},
		{"poll-overflow", "exhausted"},
		{"deadline-overflow", "exhausted"},
		{"duplicate-completion", "completion"},
		{"canceled-call", ""},
		{"poll-deadline", ""},
	} {
		t.Run("translated-"+test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			result, err := Run(ctx, built.Artifacts[0], simrt.RunConfig{Test: "TestFault", Seed: 42, ExtraEnv: []string{"FAULT_CASE=" + test.name}})
			if err != nil {
				t.Fatal(err)
			}
			if result.Failed != (test.want != "") || !strings.Contains(result.Err, test.want) || strings.Contains(result.Err, "fault assertion") || strings.Contains(result.Err, "unwound") {
				t.Fatalf("fault %s: %+v\n%s", test.name, result, result.LogOutput)
			}
			if test.name == "cleanup-all" {
				for _, want := range []string{"release fault", "detach failure", "close failure"} {
					if !strings.Contains(result.Err, want) {
						t.Errorf("lost %s: %s", want, result.Err)
					}
				}
			}
			if test.name == "rollback-errors" && !strings.Contains(result.Err, "abort failure") {
				t.Fatal("lost abort failure", result.Err)
			}
			found := false
			for _, line := range bytes.Split(result.LogOutput, []byte{'\n'}) {
				var record struct {
					Message                     string `json:"msg"`
					Released                    bool   `json:"released"`
					Transactions, Polls, Leases int
					Operations                  []string
				}
				if json.Unmarshal(line, &record) != nil || record.Message != "fault-cleanup" {
					continue
				}
				found = true
				if !record.Released || record.Transactions != 0 || record.Polls != 0 || record.Leases != 0 {
					t.Fatalf("fault leaked ownership: %+v", record)
				}
				if !strings.HasPrefix(test.name, "open-") && !strings.HasPrefix(test.name, "controls-") {
					if len(record.Operations) < 2 || !slices.Equal(record.Operations[len(record.Operations)-2:], []string{"net:close", "fs:close"}) {
						t.Fatalf("session cleanup lost reverse ordering: %v", record.Operations)
					}
				}
				t.Logf("%s cleanup: %v", test.name, record.Operations)
			}
			if !found {
				t.Fatalf("missing translated cleanup evidence: %s", result.LogOutput)
			}
		})
	}
	var previous simrt.RunResult
	for _, name := range []string{"random-baseline", "random-net"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		result, err := Run(ctx, built.Artifacts[0], simrt.RunConfig{Test: "TestFault", Seed: 42, ExtraEnv: []string{"FAULT_CASE=" + name}})
		cancel()
		if err != nil || result.Failed {
			t.Fatalf("random isolation: %v %+v", err, result)
		}
		if name == "random-net" && (!bytes.Equal(previous.LogOutput, result.LogOutput) || !bytes.Equal(previous.Checksum, result.Checksum)) {
			t.Fatal("network draws advanced fs/scheduler replay")
		}
		previous = result
	}
}

const faultApplication = `//go:build sim
package faultfixture
import("os";"testing";"github.com/neria-cloud/gosim/internal/control";"github.com/neria-cloud/gosim/internal/kernel")
func TestFault(t *testing.T){
 defer func(){if recover()!=nil{panic("fault unwound application")}}()
 inv:=kernel.Invocation{Route:kernel.RouteID(0xfffffff0),Args:[5]any{os.Getenv("FAULT_CASE")}}
 if err:=control.Submit(inv.Route,&inv);err!=nil{t.Fatal(err)}
}
`
