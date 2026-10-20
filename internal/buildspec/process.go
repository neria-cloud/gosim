package buildspec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const (
	canceledExitCode   = 130
	signalExitCodeBase = 128
)

// RunProcess owns the process group, forwards cancellation, then reaps the
// leader. A bounded grace period lets workers clean up private build directories.
func RunProcess(ctx context.Context, cmd *exec.Cmd) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd.Cancel = nil
	cmd.WaitDelay = 0
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			_ = cmd.Process.Kill()
		}
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
			// The leader may exit before children that ignored SIGINT. Do not
			// leave those descendants running after the invocation was canceled.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		case <-timer.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return ctx.Err()
	}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return canceledExitCode
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return signalExitCodeBase + int(status.Signal())
		}
		return exit.ExitCode()
	}
	if errors.Is(err, os.ErrProcessDone) {
		return 0
	}
	return 1
}
