//go:build !sim

package metatesting

import (
	"testing"

	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func TestConstructorsOwnIndependentChildren(t *testing.T) {
	t.Parallel()
	var parent, duplicate, child *MetaT
	pkg := gosimtool.Module + "/internal/tests/rt"
	t.Run("parent", func(t *testing.T) {
		parent = ForOtherPackage(t, pkg, Options{})
		duplicate = ForOtherPackage(t, pkg, Options{})
		if duplicate == parent || duplicate.cmd.Process.Pid == parent.cmd.Process.Pid {
			t.Fatal("constructors shared a worker")
		}
		t.Run("child", func(t *testing.T) {
			child = ForOtherPackage(t, pkg, Options{})
			if _, err := child.ListTests(); err != nil {
				t.Fatal(err)
			}
		})
		if child.cmd.ProcessState == nil {
			t.Fatal("child cleanup did not reap worker")
		}
		if parent.cmd.ProcessState != nil {
			t.Fatal("child cleanup reaped parent worker")
		}
		result, err := parent.Run(t, &RunConfig{Test: "TestHello", Seed: 1})
		if err != nil {
			t.Fatal(err)
		}
		if result.Failed {
			t.Fatal(result.Err)
		}
		if err := duplicate.Close(); err != nil {
			t.Fatal(err)
		}
		if err := duplicate.Close(); err != nil {
			t.Fatal("second close", err)
		}
	})
	for _, r := range []*MetaT{parent, duplicate, child} {
		if r.cmd.ProcessState == nil || !r.cmd.ProcessState.Exited() {
			t.Fatal("cleanup did not join worker")
		}
	}
}
