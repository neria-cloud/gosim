// Command gosim-gen generates owned version-profile adapters and transport.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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

func printUsageError() {
	fmt.Fprintln(
		os.Stderr,
		"gosim-gen requires -profile, -arches and -output; -check and -refresh-origin cannot be combined",
	)
}

func printError(err error) {
	fmt.Fprintln(os.Stderr, err)
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
	options := commandOptions{}
	flag.StringVar(
		&options.toolchain,
		"toolchain",
		"",
		"optional exact source compiler override; default uses normal Go resolution",
	)
	flag.StringVar(&options.profile, "profile", "", "destination family, e.g. go125")
	flag.StringVar(&options.arches, "arches", "", "comma-separated simulated Linux architectures")
	flag.StringVar(&options.output, "output", "", "explicit output repository root")
	flag.BoolVar(
		&options.refresh,
		"refresh-origin",
		false,
		"intentionally refresh generation provenance after checking reviewed contracts",
	)
	flag.BoolVar(&options.check, "check", false, "verify generated outputs without modifying them")
	flag.Parse()
	if invalidArguments(options, flag.NArg()) {
		printUsageError()
		os.Exit(2)
	}
	if err := run(options); err != nil {
		printError(err)
		os.Exit(1)
	}
}
