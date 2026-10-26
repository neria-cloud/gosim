//go:build !sim

package behavior_test

import (
	"bytes"
	"testing"

	"github.com/neria-cloud/gosim/metatesting"
)

func TestFlockDeterministicReplay(t *testing.T) {
	mt := metatesting.ForPackage(t, metatesting.Options{})
	for _, name := range []string{"TestFlockIndependentOpenExclusion", "TestFlockCrashReacquireAndRestoreIsolation", "TestFlockCloseWaitsForDescriptorDestruction"} {
		t.Run(name, func(t *testing.T) {
			first, err := mt.Run(t, &metatesting.RunConfig{Test: name, Seed: 3})
			if err != nil || first.Failed {
				t.Fatal("first run", err, first)
			}
			second, err := mt.Run(t, &metatesting.RunConfig{Test: name, Seed: 3})
			if err != nil || second.Failed {
				t.Fatal("second run", err, second)
			}
			if len(first.Checksum) == 0 || !bytes.Equal(first.Checksum, second.Checksum) || !bytes.Equal(first.LogOutput, second.LogOutput) {
				t.Fatal("lock replay diverged")
			}
		})
	}
}
