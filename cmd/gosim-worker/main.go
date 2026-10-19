// Command gosim-worker executes commands using its own compile-time Go family.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/neria-cloud/gosim/internal/gosimcli"
	"github.com/neria-cloud/gosim/internal/workerboot"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--worker-info" {
		if err := workerboot.WriteInfo(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(gosimcli.Run(ctx, os.Args[1:], workerboot.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}))
}
