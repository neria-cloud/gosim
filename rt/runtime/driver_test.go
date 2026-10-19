package gosimruntime

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestEngineRunCaptureAndOwnership(t *testing.T) {
	var first RunResult
	for iteration := 0; iteration < 2; iteration++ {
		e := testEngine(t)
		env := []string{"EXAMPLE=value"}
		if err := e.ConfigureRun(RunConfig{Seed: 42, ExtraEnv: env, Simtrace: "syscall"}); err != nil {
			t.Fatal(err)
		}
		env[0] = "EXAMPLE=mutated"
		if err := e.Run(func() {
			if !TraceSyscall.Enabled() || !slices.Contains(Envs(), "EXAMPLE=value") || slices.Contains(Envs(), "EXAMPLE=mutated") {
				panic("run inputs not owned")
			}
			WriteLog([]byte("captured output\n"))
		}); err != nil {
			t.Fatal(err)
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		result := e.RunResult(strings.Repeat("a", 64), "")
		if result.Failed || result.Err != "" || result.Seed != 42 || result.BuildDigest != strings.Repeat("a", 64) || result.ReplayRevision != ReplayRevision || len(result.Checksum) == 0 || !bytes.Contains(result.LogOutput, []byte("captured output")) {
			t.Fatalf("bad result %+v", result)
		}
		if TraceSyscall.Enabled() {
			t.Fatal("trace configuration leaked outside run")
		}
		if iteration == 0 {
			first = result
			first.Checksum = slices.Clone(result.Checksum)
		} else if !bytes.Equal(first.Checksum, result.Checksum) {
			t.Fatal("same seed/callback diverged")
		}
		repeated := e.RunResult(result.BuildDigest, "")
		result.Checksum[0] ^= 255
		result.LogOutput[0] ^= 255
		if !bytes.Equal(repeated.Checksum, e.RunResult(result.BuildDigest, "").Checksum) || !bytes.Equal(repeated.LogOutput, e.RunResult(result.BuildDigest, "").LogOutput) {
			t.Fatal("result exposes capture memory")
		}
	}
}

func TestEngineCaptureFailureAndState(t *testing.T) {
	e := testEngine(t)
	if err := e.ConfigureRun(RunConfig{Seed: 41}); !errors.Is(err, ErrEngineState) {
		t.Fatal(err)
	}
	if result := e.RunResult("", ""); !result.Failed {
		t.Fatal("premature result accepted")
	}
	if err := e.ConfigureRun(RunConfig{Seed: 42, Simtrace: "unknown"}); err != nil {
		t.Fatal(err)
	}
	if err := e.ConfigureRun(RunConfig{Seed: 42}); !errors.Is(err, ErrEngineState) {
		t.Fatal(err)
	}
	entered := false
	err := e.Run(func() { entered = true })
	if err == nil || !strings.Contains(err.Error(), "unknown traceflag") || entered {
		t.Fatal(err, entered)
	}
	e.Close()
	result := e.RunResult("build", "")
	if !result.Failed || !strings.Contains(result.Err, "unknown traceflag") || len(result.Checksum) != 0 {
		t.Fatalf("bad failure %+v", result)
	}
	if err := e.ConfigureRun(RunConfig{Seed: 42}); !errors.Is(err, ErrEngineState) {
		t.Fatal(err)
	}
	next := testEngine(t)
	if err := next.ConfigureRun(RunConfig{Seed: 42}); err != nil {
		t.Fatal(err)
	}
	if err := next.Run(func() {}); err != nil {
		t.Fatal(err)
	}
	next.Close()
	if result := next.RunResult("build", "translated cleanup failure"); !result.Failed || result.Err != "translated cleanup failure" {
		t.Fatal(result)
	}
}

func TestCaptureDoesNotMutateAnotherActiveEngine(t *testing.T) {
	outer := testEngine(t)
	inner := testEngine(t)
	if err := outer.ConfigureRun(RunConfig{Seed: 42, Simtrace: "syscall"}); err != nil {
		t.Fatal(err)
	}
	if err := inner.ConfigureRun(RunConfig{Seed: 42, Simtrace: "network"}); err != nil {
		t.Fatal(err)
	}
	if err := outer.Run(func() {
		if err := inner.Run(func() { panic("nested engine") }); !errors.Is(err, ErrEngineBusy) {
			panic("nested engine accepted")
		}
		if !TraceSyscall.Enabled() || TraceNetwork.Enabled() {
			panic("rejected run changed active trace flags")
		}
	}); err != nil {
		t.Fatal(err)
	}
}
