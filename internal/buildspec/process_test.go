package buildspec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const testProcessExitCode = 23

func TestRunProcessStreamsAndExit(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("/bin/sh", "-c", "cat; printf diagnostic >&2; exit 23")
	var out, diagnostics bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader("input"), &out, &diagnostics
	err := RunProcess(t.Context(), cmd)
	if ExitCode(err) != testProcessExitCode || out.String() != "input" || diagnostics.String() != "diagnostic" {
		t.Fatalf("err=%v stdout=%s stderr=%s", err, &out, &diagnostics)
	}
}

func TestRunProcessCancellationReapsGroup(t *testing.T) {
	t.Parallel()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// Both processes ignore SIGINT, exercising the bounded escalation path.
	cmd := exec.Command("/bin/sh", "-c", `trap '' INT; sleep 30 & child=$!; echo "$child"; wait "$child"`)
	cmd.Stdout = write
	done := make(chan error, 1)
	go func() { done <- RunProcess(ctx, cmd) }()
	line, err := bufio.NewReader(read).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || ExitCode(err) != interruptedExitCode {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not reap process")
	}
	// The child may briefly remain a zombie until its adopter reaps it. A
	// process listing must never show it running after group cancellation.
	output, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err == nil && !strings.HasPrefix(strings.TrimSpace(string(output)), "Z") && len(bytes.TrimSpace(output)) != 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("descendant still alive: %s", output)
	}
}

func TestRunProcessAlreadyCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cmd := exec.Command("/missing-command")
	if err := RunProcess(ctx, cmd); !errors.Is(err, context.Canceled) || cmd.Process != nil {
		t.Fatal(err)
	}
}
