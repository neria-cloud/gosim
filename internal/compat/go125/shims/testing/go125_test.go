//go:build go1.25 && !go1.26

package testing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	native "testing"
)

func TestContextCancelsBeforeCleanup(t *native.T) {
	var simulated T
	ctx := simulated.Context()
	if ctx != simulated.Context() || ctx.Err() != nil {
		t.Fatal("Context identity/state")
	}
	var order []int
	simulated.Cleanup(func() {
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Error("cleanup ran before context cancellation")
		}
		order = append(order, 1)
	})
	simulated.Cleanup(func() { order = append(order, 2) })
	if value := simulated.runCleanup(recoverAndReturnPanic); value != nil {
		t.Fatal(value)
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("cleanup order=%v", order)
	}
	var unused T
	unused.runCleanup(normalPanic)
	if !errors.Is(unused.Context().Err(), context.Canceled) {
		t.Fatal("late Context must already be canceled")
	}
}

func TestOutputAndAttributes(t *native.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	var attrs bytes.Buffer
	simulated := T{common: common{name: "test", chatty: newChattyPrinter(&attrs)}}
	writer := simulated.Output()
	if n, err := io.WriteString(writer, "first\nsec"); n != 9 || err != nil {
		t.Fatalf("write=%d,%v", n, err)
	}
	if strings.Contains(logs.String(), "sec") {
		t.Fatal("unterminated line was emitted")
	}
	io.WriteString(writer, "ond\nlast")
	simulated.Attr("platform", "simulated")
	if !strings.Contains(attrs.String(), "=== ATTR  test platform simulated") {
		t.Fatal(attrs.String())
	}
	simulated.runCleanup(normalPanic)
	for _, line := range []string{"first", "second", "last"} {
		if !strings.Contains(logs.String(), line) {
			t.Fatalf("missing %q: %s", line, logs.String())
		}
	}
	simulated.Attr("invalid key", "value")
	if !simulated.Failed() {
		t.Fatal("invalid attribute did not fail test")
	}
}

func TestFilesystemHelpersDelegateAndRestore(t *native.T) {
	// Native unit invocation verifies helper cleanup only. In generated code these
	// same os imports are translated and must pass the filesystem feature gate.
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var simulated T
	t.Cleanup(func() { simulated.runCleanup(normalPanic) })
	simulated.Setenv("GOSIM_SHIM_TEST_KEY", "value")
	simulated.Chdir(directory)
	temporary := simulated.TempDir()
	if value := os.Getenv("GOSIM_SHIM_TEST_KEY"); value != "value" {
		t.Fatal(value)
	}
	if cwd, err := os.Getwd(); err != nil || cwd != directory {
		t.Fatalf("cwd=%s err=%v", cwd, err)
	}
	if _, err := os.Stat(temporary); err != nil {
		t.Fatal(err)
	}
	simulated.runCleanup(normalPanic)
	if cwd, err := os.Getwd(); err != nil || cwd != original {
		t.Fatalf("restored cwd=%s err=%v", cwd, err)
	}
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatalf("TempDir survived cleanup: %v", err)
	}
}

func TestParallelEnvironmentGuard(t *native.T) {
	simulated := T{common: common{isParallel: true}}
	defer func() {
		if recover() == nil {
			t.Error("parallel environment mutation accepted")
		}
	}()
	simulated.Setenv("GOSIM_SHIM_TEST_KEY", "should not be written")
}

func TestInitializationQueries(t *native.T) {
	Init()
	if CoverMode() != "" || !Testing() || !Verbose() || Short() {
		t.Fatal("runner query contract")
	}
}
