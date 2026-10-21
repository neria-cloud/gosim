//go:build go1.25

// Package exec exposes process declarations but rejects process execution.
package exec

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

var (
	ErrNotFound  = errors.New("executable file not found")
	ErrDot       = errors.New("cannot run executable found relative to current directory")
	ErrWaitDelay = errors.New("exec: WaitDelay expired before I/O complete")
)

type Error struct {
	Name string
	Err  error
}

func (e *Error) Error() string { return "exec: " + e.Name + ": " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

type ExitError struct {
	*os.ProcessState
	Stderr []byte
}

func (e *ExitError) Error() string { return e.ProcessState.String() }

type Cmd struct {
	Path           string
	Args, Env      []string
	Dir            string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	ExtraFiles     []*os.File
	SysProcAttr    *syscall.SysProcAttr
	Process        *os.Process
	ProcessState   *os.ProcessState
	Err            error
	Cancel         func() error
	WaitDelay      time.Duration
}

func unsupported() {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "os/exec", Message: "native processes are not available in simulation"})
}
func Command(name string, arg ...string) *Cmd                             { unsupported(); return nil }
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd { unsupported(); return nil }
func LookPath(file string) (string, error)                                { unsupported(); return "", nil }
func (c *Cmd) String() string                                             { return c.Path }
func (c *Cmd) Run() error                                                 { unsupported(); return nil }
func (c *Cmd) Start() error                                               { unsupported(); return nil }
func (c *Cmd) Wait() error                                                { unsupported(); return nil }
func (c *Cmd) Output() ([]byte, error)                                    { unsupported(); return nil, nil }
func (c *Cmd) CombinedOutput() ([]byte, error)                            { unsupported(); return nil, nil }
func (c *Cmd) StdinPipe() (io.WriteCloser, error)                         { unsupported(); return nil, nil }
func (c *Cmd) StdoutPipe() (io.ReadCloser, error)                         { unsupported(); return nil, nil }
func (c *Cmd) StderrPipe() (io.ReadCloser, error)                         { unsupported(); return nil, nil }
func (c *Cmd) Environ() []string                                          { unsupported(); return nil }
