package gosimcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSuiteWorkerProcess(t *testing.T) {
	if os.Getenv("GOSIM_SUITE_TEST_CHILD") != "1" {
		return
	}
	dir, name := os.Getenv("GOSIM_SUITE_TEST_DIR"), os.Getenv("GOSIM_SUITE_TEST_NAME")
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
		os.Exit(2)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			os.Exit(3)
		}
		time.Sleep(time.Millisecond)
	}
	fmt.Print(name + "\n")
	if name == "1" {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestSuiteCommandsOverlapWithinBudgetAndKeepAllResults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var commands []*exec.Cmd
	for i := range 3 {
		cmd := exec.Command(executable, "-test.run=^TestSuiteWorkerProcess$")
		cmd.Env = append(
			os.Environ(),
			"GOSIM_SUITE_TEST_CHILD=1",
			"GOSIM_SUITE_TEST_DIR="+dir,
			"GOSIM_SUITE_TEST_NAME="+strconv.Itoa(i),
		)
		commands = append(commands, cmd)
	}
	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)
	go func() { output, err := runTestCommands(t.Context(), commands, 2); done <- result{output, err} }()
	// Always release and reap workers before TempDir cleanup, including failure.
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "release"), nil, 0o600)
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Error("workers did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, a := os.Stat(filepath.Join(dir, "0"))
		_, b := os.Stat(filepath.Join(dir, "1"))
		if a == nil && b == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("two independent workers did not overlap")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(dir, "2")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("worker budget exceeded", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		done <- got // cleanup joins even after assertions fail
		if !bytes.Equal(got.output, []byte("0\n1\n2\n")) {
			t.Fatalf("lost/reordered logs: %q", got.output)
		}
		var exit *exec.ExitError
		if !errors.As(got.err, &exit) || exit.ExitCode() != 1 {
			t.Fatalf("lost failed worker: %v", got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("workers did not complete")
	}
}

func TestSuiteCancellationStopsCommands(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	if _, err := runTestCommands(ctx, []*exec.Cmd{cmd}, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("suite did not propagate cancellation", err)
	}
}

func TestSuiteConcurrencyBudget(t *testing.T) {
	t.Parallel()
	for budget := 1; budget <= 16; budget++ {
		flags := nativeTestParallelFlags(budget)
		packages, _ := strconv.Atoi(strings.TrimPrefix(flags[0], "-p="))
		tests, _ := strconv.Atoi(strings.TrimPrefix(flags[1], "-parallel="))
		if packages < 1 || tests < 1 || packages*tests > budget {
			t.Fatalf("budget %d: %v", budget, flags)
		}
		if budget > 1 && tests < 2 {
			t.Fatalf("parallel tests disabled: %v", flags)
		}
	}
	if _, err := runTestCommands(t.Context(), nil, 0); err == nil {
		t.Fatal("accepted zero budget")
	}
	if out, err := runTestCommands(t.Context(), nil, 2); err != nil || len(out) != 0 {
		t.Fatal(string(out), err)
	}
}

func TestCertificationSelection(t *testing.T) {
	t.Parallel()
	for _, arch := range []string{"amd64", "arm64"} {
		for _, race := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/race=%t", arch, race), func(t *testing.T) {
				t.Parallel()
				err := validateCertificationSelection(arch, race)
				if (err != nil) != (arch != "arm64" || race) {
					t.Fatalf("unexpected certification policy: %v", err)
				}
			})
		}
	}
}
