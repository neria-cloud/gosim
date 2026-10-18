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

func GenerateProfile(tc buildspec.Toolchain, profileID string, arches []string, output string) error {
	return routegen.GenerateProfile(tc, profileID, arches, output)
}

func main() {
	toolchain := flag.String("toolchain", "", "exact source toolchain, e.g. go1.25.14")
	profile := flag.String("profile", "", "destination family, e.g. go125")
	arches := flag.String("arches", "", "comma-separated simulated Linux architectures")
	output := flag.String("output", "", "explicit output repository root")
	flag.Parse()
	if flag.NArg() != 0 || *toolchain == "" || *profile == "" || *arches == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "gosim-gen requires -toolchain, -profile, -arches and -output, with no positional arguments")
		os.Exit(2)
	}
	tc, err := buildcfg.Resolve(*toolchain)
	if err == nil {
		err = GenerateProfile(tc, *profile, strings.Split(*arches, ","), *output)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
