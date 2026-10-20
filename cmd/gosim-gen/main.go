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

func GenerateProfile(
	tc buildspec.Toolchain,
	profileID string,
	arches []string,
	output string,
	options routegen.GenerateOptions,
) error {
	return routegen.GenerateProfile(tc, profileID, arches, output, options)
}

func main() {
	toolchain := flag.String(
		"toolchain",
		"",
		"optional exact source compiler override; default uses normal Go resolution",
	)
	profile := flag.String("profile", "", "destination family, e.g. go125")
	arches := flag.String("arches", "", "comma-separated simulated Linux architectures")
	output := flag.String("output", "", "explicit output repository root")
	refresh := flag.Bool(
		"refresh-origin",
		false,
		"intentionally refresh generation provenance after checking reviewed contracts",
	)
	check := flag.Bool("check", false, "verify generated outputs without modifying them")
	flag.Parse()
	if (*check && *refresh) || flag.NArg() != 0 || *profile == "" || *arches == "" || *output == "" {
		fmt.Fprintln(
			os.Stderr,
			"gosim-gen requires -profile, -arches and -output; -check and -refresh-origin cannot be combined",
		)
		os.Exit(2)
	}
	tc, err := buildcfg.Resolve(*toolchain)
	if err == nil {
		if *check {
			err = routegen.CheckProfile(tc, *profile, strings.Split(*arches, ","), *output)
		} else {
			err = GenerateProfile(tc, *profile, strings.Split(*arches, ","), *output, routegen.GenerateOptions{RefreshOrigin: *refresh})
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
