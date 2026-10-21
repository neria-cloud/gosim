//go:build !sim

package behavior_test

import (
	"bytes"
	"testing"

	"github.com/neria-cloud/gosim/metatesting"
)

func TestProtobufStatusReplayAfterInterveningRuns(t *testing.T) {
	runner := metatesting.ForPackage(t, metatesting.Options{})
	var first *metatesting.RunResult
	for _, seed := range []int64{1, 2, 3, 1} {
		result, err := runner.Run(t, &metatesting.RunConfig{Test: "TestProtobufStatusPerMachine", Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		if result.Failed || result.Err != "" || len(result.Checksum) == 0 || result.BuildDigest == "" || result.ReplayRevision == "" {
			t.Fatalf("descriptor run seed %d: %+v", seed, result)
		}
		if first == nil {
			first = result
		} else if seed == 1 {
			if result.BuildDigest != first.BuildDigest || result.ReplayRevision != first.ReplayRevision || !bytes.Equal(result.Checksum, first.Checksum) {
				t.Fatalf("seed 1 descriptor replay changed after intervening runs: first=%x again=%x", first.Checksum, result.Checksum)
			}
		}
	}
	fresh := metatesting.ForPackage(t, metatesting.Options{})
	result, err := fresh.Run(t, &metatesting.RunConfig{Test: "TestProtobufStatusPerMachine", Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed || result.Err != "" || len(result.Checksum) == 0 || result.BuildDigest != first.BuildDigest || result.ReplayRevision != first.ReplayRevision || !bytes.Equal(result.Checksum, first.Checksum) {
		t.Fatalf("fresh-worker descriptor replay changed: first=%x fresh=%x result=%+v", first.Checksum, result.Checksum, result)
	}
}
