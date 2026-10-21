//go:build !sim

package etcd_test

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"

	"github.com/neria-cloud/gosim/metatesting"
)

var etcdPhasePattern = regexp.MustCompile(`cluster elapsed=[^;]+; before=\{Attempts:([0-9]+) Successes:([0-9]+)\} fault=\{Attempts:([0-9]+) Successes:([0-9]+)\} after=\{Attempts:([0-9]+) Successes:([0-9]+)\}`)

type etcdPhases [6]int

func firstEtcdLogDifference(left, right []byte) string {
	a, b := bytes.Split(left, []byte("\n")), bytes.Split(right, []byte("\n"))
	for i := 0; i < len(a) && i < len(b); i++ {
		if !bytes.Equal(a[i], b[i]) {
			return string(bytes.Join([][]byte{[]byte(strconv.Itoa(i)), a[i], b[i]}, []byte(" | ")))
		}
	}
	return "log lengths " + strconv.Itoa(len(a)) + " and " + strconv.Itoa(len(b))
}

func readEtcdPhases(t *testing.T, output []byte) etcdPhases {
	t.Helper()
	matches := etcdPhasePattern.FindSubmatch(output)
	if len(matches) != 7 {
		t.Fatal("missing semantic cluster phase result")
	}
	var result etcdPhases
	for i, value := range matches[1:] {
		parsed, err := strconv.Atoi(string(value))
		if err != nil {
			t.Fatal(err)
		}
		result[i] = parsed
	}
	if result[1] == 0 || result[5] == 0 || result[0] < result[1] || result[4] < result[5] {
		t.Fatalf("invalid cluster progress: %+v", result)
	}
	return result
}

func TestEtcdReplayAfterInterveningRuns(t *testing.T) {
	const pkg = "github.com/neria-cloud/gosim/examples/etcd"
	runner := metatesting.ForOtherPackage(t, pkg, metatesting.Options{})
	var first *metatesting.RunResult
	var firstPhases etcdPhases
	for _, seed := range []int64{1, 2, 3, 1} {
		result, err := runner.Run(t, &metatesting.RunConfig{Test: "TestEtcd", Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		if result.Failed || result.Err != "" || len(result.Checksum) == 0 || result.BuildDigest == "" || result.ReplayRevision == "" {
			t.Fatalf("etcd seed %d failed or lost identity: %+v", seed, result)
		}
		phases := readEtcdPhases(t, result.LogOutput)
		if first == nil {
			first, firstPhases = result, phases
		} else if seed == 1 && (result.BuildDigest != first.BuildDigest || result.ReplayRevision != first.ReplayRevision || !bytes.Equal(result.Checksum, first.Checksum) || phases != firstPhases) {
			t.Fatalf("etcd seed 1 replay changed after intervening runs: first=%x %+v; again=%x %+v; first log difference: %s", first.Checksum, firstPhases, result.Checksum, phases, firstEtcdLogDifference(first.LogOutput, result.LogOutput))
		}
	}
	fresh := metatesting.ForOtherPackage(t, pkg, metatesting.Options{})
	result, err := fresh.Run(t, &metatesting.RunConfig{Test: "TestEtcd", Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed || result.Err != "" || len(result.Checksum) == 0 || result.BuildDigest != first.BuildDigest || result.ReplayRevision != first.ReplayRevision || !bytes.Equal(result.Checksum, first.Checksum) || readEtcdPhases(t, result.LogOutput) != firstPhases {
		t.Fatalf("fresh-worker etcd replay changed: first=%x %+v; fresh=%x", first.Checksum, firstPhases, result.Checksum)
	}
}
