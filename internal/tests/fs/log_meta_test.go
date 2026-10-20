//go:build !sim

package behavior_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/neria-cloud/gosim/internal/gosimlog"
	"github.com/neria-cloud/gosim/metatesting"
)

func formatLogsWithExtra(logs []*gosimlog.Log) []string {
	// TODO: share this code with prettylog somehow?
	lines := make([]string, 0, len(logs))
	for _, log := range logs {
		line := fmt.Sprintf("%d %s/%d %s %s", log.Step, log.Machine, log.Goroutine, log.Level, log.Msg)
		for _, kv := range log.Unknown {
			line += fmt.Sprintf(" %s=%s", kv.Key, kv.Value)
		}
		lines = append(lines, line)
	}
	return lines
}

func TestLogTraceSyscall(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})

	// no logs without simtrace
	run, err := mt.Run(t, &metatesting.RunConfig{
		Test: "TestLogTraceSyscall",
		Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(metatesting.SimplifyParsedLog(metatesting.ParseLog(run.LogOutput)), []string(nil)); diff != "" {
		t.Error("diff", diff)
	}

	// syscalls with simtrace
	run, err = mt.Run(t, &metatesting.RunConfig{
		Test:     "TestLogTraceSyscall",
		Seed:     1,
		Simtrace: "syscall",
	})
	if err != nil {
		t.Fatal(err)
	}

	// TODO: support snapshotting these logs?
	if diff := cmp.Diff(formatLogsWithExtra(gosimlog.ParseLog(run.LogOutput)), []string{
		"1 main/2 INFO call FcntlInt",
		"2 main/2 INFO ret  FcntlInt",
		"3 main/2 INFO call FcntlInt",
		"4 main/2 INFO ret  FcntlInt",
		"5 main/2 INFO call FcntlInt",
		"6 main/2 INFO ret  FcntlInt",
		"7 main/3 INFO call Require",
		"8 main/3 INFO ret  Require",
		"9 main/3 INFO call Openat",
		"10 main/3 INFO ret  Openat",
		"11 main/3 INFO call FcntlInt",
		"12 main/3 INFO ret  FcntlInt",
		"13 main/3 INFO call Write",
		"14 main/3 INFO ret  Write",
		"15 main/3 INFO call Close",
		"16 main/3 INFO ret  Close",
	}); diff != "" {
		t.Error("diff", diff)
	}
}
