//go:build sim

package behavior_test

import (
	"errors"
	"os"
	"testing"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func TestRecoveryLimitAndIndependentOwners(t *testing.T) {
	source := testsupport.Start(t, func() {
		testsupport.Check(t, os.WriteFile("pending", []byte("retained"), 0o600))
		gosim.CrashSelf()
	})
	testsupport.Wait(t, source)
	if _, err := simfs.CrashStates(source, simfs.EnumerateOptions{}); !errors.Is(err, gosim.ErrInvalidConfig) {
		t.Fatal("zero budget", err)
	}
	iterator, err := simfs.CrashStates(source, simfs.EnumerateOptions{MaxStates: 1})
	testsupport.Check(t, err)
	snapshot, ok, err := iterator.Next()
	testsupport.Check(t, err)
	if !ok {
		t.Fatal("missing first snapshot")
	}
	for range 2 {
		if _, ok, err := iterator.Next(); ok || !errors.Is(err, simfs.ErrEnumerationLimit) {
			t.Fatal("limit confused with exhaustion", ok, err)
		}
	}
	testsupport.Check(t, iterator.Close())
	testsupport.Check(t, iterator.Close())
	if _, _, err := iterator.Next(); !errors.Is(err, gosim.ErrInvalidState) {
		t.Fatal("closed iterator usable", err)
	}
	testsupport.Check(t, source.Destroy())
	first, err := gosim.NewMachine(
		gosim.MachineConfig{Main: func() { testsupport.Check(t, os.WriteFile("pending", []byte("changed"), 0o600)) }},
	)
	testsupport.Check(t, err)
	second, err := gosim.NewMachine(gosim.MachineConfig{Main: func() {
		data, err := os.ReadFile("pending")
		testsupport.Check(t, err)
		if string(data) != "retained" {
			t.Fatalf("restore shared mutable state: %q", data)
		}
	}})
	testsupport.Check(t, err)
	testsupport.Check(t, simfs.Restore(first, snapshot))
	testsupport.Check(t, simfs.Restore(second, snapshot))
	testsupport.Check(t, snapshot.Close())
	testsupport.Check(t, snapshot.Close())
	if err := simfs.Restore(second, snapshot); !errors.Is(err, gosim.ErrInvalidState) {
		t.Fatal("closed snapshot usable", err)
	}
	testsupport.Check(t, first.Start())
	testsupport.Wait(t, first)
	testsupport.Check(t, second.Start())
	testsupport.Wait(t, second)
	testsupport.Check(t, first.Destroy())
	testsupport.Check(t, second.Destroy())
}
