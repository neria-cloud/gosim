package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

type commandRunner struct {
	generate func(commandOptions) error
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return (commandRunner{generate: run}).dispatch(ctx, args, stdout, stderr)
}

func (r commandRunner) dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "profiles":
			return runProfiles(ctx, args[1:], stdout, stderr)
		case "compare":
			return runCompare(ctx, args[1:], stdout, stderr)
		case "observe":
			return runObserve(ctx, args[1:], stdout, stderr)
		}
	}
	return r.generation(args, stdout, stderr)
}

func parseFlags(flags *flag.FlagSet, args []string, stdout, stderr io.Writer) int {
	flags.SetOutput(io.Discard)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(stdout)
			flags.Usage()
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments:", flags.Args())
		return 2
	}
	return -1 // Successfully parsed; the caller continues execution.
}

func (r commandRunner) generation(args []string, stdout, stderr io.Writer) int {
	options := commandOptions{}
	flags := flag.NewFlagSet("gosim-gen [profiles|observe|compare] or generation flags", flag.ContinueOnError)
	flags.StringVar(&options.toolchain, "toolchain", "", "optional exact source compiler override")
	flags.StringVar(&options.profile, "profile", "", "stable Go family, e.g. go125 (all stable 1.25 patches)")
	flags.StringVar(&options.arches, "arches", "", "comma-separated simulated Linux architectures")
	flags.StringVar(&options.output, "output", "", "explicit output repository root")
	flags.BoolVar(&options.refresh, "refresh-origin", false, "explicitly refresh provenance after contract checks")
	flags.BoolVar(&options.check, "check", false, "verify generated outputs without modifying them")
	if code := parseFlags(flags, args, stdout, stderr); code >= 0 {
		return code
	}
	if invalidArguments(options, flags.NArg()) {
		fmt.Fprintln(stderr, "requires -profile, -arches and -output; -check and -refresh-origin cannot be combined")
		return 2
	}
	if err := r.generate(options); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runProfiles(_ context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gosim-gen profiles", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root containing family definitions")
	flags.Bool("check", false, "verify support tables, gates and revisions without writing")
	if code := parseFlags(flags, args, stdout, stderr); code >= 0 {
		return code
	}
	if *root == "" {
		fmt.Fprintln(stderr, "profiles requires a nonempty -root")
		return 2
	}
	return unavailable("profiles", stderr)
}

func runCompare(_ context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gosim-gen compare", flag.ContinueOnError)
	parent := flags.String("parent", "", "immediate parent stable Go family, e.g. go126")
	flags.String("parent-observation", "", "optional complete parent API observation")
	output := inspectionFlags(flags)
	if code := parseFlags(flags, args, stdout, stderr); code >= 0 {
		return code
	}
	if *parent == "" || *output == "" {
		fmt.Fprintln(stderr, "compare requires -parent and -output (a new directory)")
		return 2
	}
	return unavailable("compare", stderr)
}

func runObserve(_ context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gosim-gen observe", flag.ContinueOnError)
	profile := flags.String("profile", "", "parent stable Go family to observe at its recorded compiler")
	output := inspectionFlags(flags)
	if code := parseFlags(flags, args, stdout, stderr); code >= 0 {
		return code
	}
	if *profile == "" || *output == "" {
		fmt.Fprintln(stderr, "observe requires -profile and -output (a new file)")
		return 2
	}
	return unavailable("observe", stderr)
}

func inspectionFlags(flags *flag.FlagSet) *string {
	flags.String("arches", "amd64,arm64", "comma-separated simulated Linux source architectures")
	flags.String("toolchain", "", "optional exact compiler equality assertion; must match running compiler")
	return flags.String("output", "", "required new output destination; existing destinations are refused")
}

// Removed when the maintenance and inspection handlers land in Tasks 18 and 27.
func unavailable(command string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "%s: command not yet implemented\n", command)
	return 1
}
