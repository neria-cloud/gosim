//go:build !sim

package metatesting

import (
	"testing"

	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func TestRunnerLastOwnerReapsProcess(t *testing.T) {
	var runner *MetaT
	pkg := gosimtool.Module + "/internal/tests/behavior"
	t.Run("parent", func(t *testing.T) {
		runner = ForOtherPackage(t, pkg)
		if ForOtherPackage(t, pkg) != runner {
			t.Fatal("duplicate acquisition replaced worker")
		}
		t.Run("child", func(t *testing.T) {
			if ForOtherPackage(t, pkg) != runner {
				t.Fatal("live owners did not share worker")
			}
			if _, err := runner.ListTests(); err != nil {
				t.Fatal(err)
			}
		})
		if runner.cmd.ProcessState != nil {
			t.Fatal("child cleanup reaped parent's worker")
		}
		result, err := runner.Run(t, &RunConfig{Test: "TestHello", Seed: 1})
		if err != nil {
			t.Fatal(err)
		}
		if result.Failed {
			t.Fatal(result.Err)
		}
	})
	if runner.cmd.ProcessState == nil || !runner.cmd.ProcessState.Exited() {
		t.Fatal("last owner did not join worker and stderr copier")
	}
	runnerMapMu.Lock()
	defer runnerMapMu.Unlock()
	if runnerMap[pkg] != nil || len(runner.owners) != 0 {
		t.Fatal("closed worker retained in registry")
	}
}
