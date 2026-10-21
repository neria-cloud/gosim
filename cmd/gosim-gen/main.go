// Command gosim-gen generates owned version-profile adapters and transport.
package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/routegen"
)

type generationRequest struct {
	toolchain buildspec.Toolchain
	profileID string
	arches    []string
	output    string
	options   routegen.GenerateOptions
}

type commandOptions struct {
	toolchain string
	profile   string
	arches    string
	output    string
	check     bool
	refresh   bool
}

func generateProfile(request generationRequest) error {
	return routegen.GenerateProfile(
		request.toolchain,
		request.profileID,
		request.arches,
		request.output,
		request.options,
	)
}

func invalidArguments(options commandOptions, positionalArgs int) bool {
	return options.output == "" || options.arches == "" || options.profile == "" ||
		positionalArgs != 0 || (options.check && options.refresh)
}

func run(options commandOptions) error {
	tc, err := buildcfg.Resolve(options.toolchain)
	if err != nil {
		return err
	}
	archList := strings.Split(options.arches, ",")
	if options.check {
		return routegen.CheckProfile(tc, options.profile, archList, options.output)
	}
	return generateProfile(generationRequest{
		toolchain: tc,
		profileID: options.profile,
		arches:    archList,
		output:    options.output,
		options:   routegen.GenerateOptions{RefreshOrigin: options.refresh},
	})
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runCLI(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
