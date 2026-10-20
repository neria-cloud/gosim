// Command gosim resolves Go in the caller context and starts a matching worker.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/neria-cloud/gosim/internal/workerboot"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(workerboot.Main(ctx, os.Args[1:], workerboot.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}))
}
