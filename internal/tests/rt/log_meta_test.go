//go:build !sim

package behavior_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/neria-cloud/gosim/internal/gosimlog"
	"github.com/neria-cloud/gosim/metatesting"
)

func parseLog(t *testing.T, log []byte) []map[string]any {
	var logs []map[string]any
	for _, line := range bytes.Split(log, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("===")) || bytes.HasPrefix(line, []byte("---")) {
			// XXX: test this also, later?
			continue
		}
		if len(line) == 0 {
			logs = append(logs, nil)
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal(line, &msg); err != nil {
			t.Fatal(line, err)
		}
		delete(msg, "source")
		logs = append(logs, msg)
	}
	return logs
}

func TestLogMachineGoroutineTime(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})
	run, err := mt.Run(t, &metatesting.RunConfig{
		Test: "TestLogMachineGoroutineTime",
		Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual := parseLog(t, run.LogOutput)
	expected := parseLog(
		t,
		[]byte(
			`{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"hi there","machine":"main","goroutine":3,"step":1}
{"time":"2020-01-15T14:10:13.000001234Z","level":"INFO","msg":"hey","machine":"inside","goroutine":5,"step":2}
{"time":"2020-01-15T14:10:23.000001234Z","level":"INFO","msg":"propagated","machine":"inside","goroutine":6,"step":3}
{"time":"2020-01-15T14:10:23.000001234Z","level":"WARN","msg":"foo","machine":"inside","bar":"baz","counter":20,"goroutine":6,"step":4}
`,
		),
	)

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error("diff", diff)
	}
}

func TestLogSLog(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})
	run, err := mt.Run(t, &metatesting.RunConfig{
		Test: "TestLogSLog",
		Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual := parseLog(t, run.LogOutput)
	expected := parseLog(
		t,
		[]byte(
			`{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"hello 10","machine":"main","goroutine":3,"step":1}
`,
		),
	)

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error("diff", diff)
	}
}

func TestLogStdoutStderr(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})
	run, err := mt.Run(t, &metatesting.RunConfig{
		Test: "TestLogStdoutStderr",
		Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual := parseLog(t, run.LogOutput)
	expected := parseLog(
		t,
		[]byte(
			`{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"hello","machine":"main","method":"stdout","goroutine":3,"step":1}
{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"goodbye","machine":"main","method":"stderr","goroutine":3,"step":2}
{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"same goroutine log","machine":"main","goroutine":3,"step":3}
{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"same goroutine slog","machine":"main","goroutine":3,"step":4}
`,
		),
	)

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error("diff", diff)
	}
}

func TestLogDuringInit(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})
	run, err := mt.Run(t, &metatesting.RunConfig{
		Test: "TestLogDuringInit",
		Seed: 1,
		ExtraEnv: []string{
			"LOGDURINGINIT=1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	actual := parseLog(t, run.LogOutput)
	expected := parseLog(
		t,
		[]byte(
			`{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"hello","machine":"main","method":"stdout","goroutine":2,"step":1}
{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"2020/01/15 14:10:03 INFO help","machine":"main","method":"stderr","goroutine":2,"step":2}
{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"hello","machine":"logm","method":"stdout","goroutine":4,"step":3}
{"time":"2020-01-15T14:10:03.000001234Z","level":"INFO","msg":"2020/01/15 14:10:03 INFO help","machine":"logm","method":"stderr","goroutine":4,"step":4}
`,
		),
	)

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error("diff", diff)
	}
}

type MsgAndStack struct {
	Msg   string
	Stack []string
}

func extractStacksUntilTest(logs []*gosimlog.Log) []MsgAndStack {
	var all []MsgAndStack
	for _, log := range logs {
		msg := log.Msg
		var stack []string
		for _, frame := range log.Stackframes {
			f := frame.Function
			f = f[strings.LastIndex(f, "/")+1:]
			f = f[strings.Index(f, ".")+1:]
			stack = append(stack, f)
			if strings.HasPrefix(f, "ImplTest") {
				break
			}
		}
		all = append(all, MsgAndStack{
			Msg:   msg,
			Stack: stack,
		})
	}
	return all
}

func TestLogTraceStacks(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})

	// no stacks without simtrace stack
	run, err := mt.Run(t, &metatesting.RunConfig{
		Test: "TestLogTraceStacks",
		Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(extractStacksUntilTest(gosimlog.ParseLog(run.LogOutput)), []MsgAndStack{
		{
			Msg: "hello from log",
		},
		{
			Msg: "hello from slog",
		},
		{
			Msg: "hello from logger",
		},
		{
			Msg: "hello from slog",
		},
		{
			Msg: "hello from logger",
		},
	}); diff != "" {
		t.Error("diff", diff)
	}

	// stacks with simtrace stack
	run, err = mt.Run(t, &metatesting.RunConfig{
		Test:     "TestLogTraceStacks",
		Seed:     1,
		Simtrace: "stack",
	})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(extractStacksUntilTest(gosimlog.ParseLog(run.LogOutput)), []MsgAndStack{
		{
			Msg:   "hello from log",
			Stack: []string{"ImplTestLogTraceStacks"},
		},
		{
			Msg:   "hello from slog",
			Stack: []string{"c", "b", "a", "ImplTestLogTraceStacks"},
		},
		{
			Msg:   "hello from logger",
			Stack: []string{"c", "b", "a", "ImplTestLogTraceStacks"},
		},
		{
			Msg:   "hello from slog",
			Stack: []string{"c", "b", "a", "a", "a", "a", "ImplTestLogTraceStacks"},
		},
		{
			Msg:   "hello from logger",
			Stack: []string{"c", "b", "a", "a", "a", "a", "ImplTestLogTraceStacks"},
		},
	}); diff != "" {
		t.Error("diff", diff)
	}
}

func TestLogBacktraceFor(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})

	// stacks with simtrace stack
	run, err := mt.Run(t, &metatesting.RunConfig{
		Test: "TestLogBacktraceFor",
		Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(extractStacksUntilTest(gosimlog.ParseLog(run.LogOutput)), []MsgAndStack{
		{
			Msg:   "g0",
			Stack: []string{"GetStacktraceFor", "ImplTestLogBacktraceFor"},
		},
		{
			Msg: "g1",
			Stack: []string{
				"(*goroutine).park", "(*goroutine).wait", "Select", "testBacktraceForB",
				"testBacktraceForA", "ImplTestLogBacktraceFor.func1",
			},
		},
	}); diff != "" {
		t.Error("diff", diff)
	}
}
