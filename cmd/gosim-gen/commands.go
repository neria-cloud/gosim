package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/neria-cloud/gosim/internal/compat/audit"

	"github.com/neria-cloud/gosim/internal/compat/profiles"
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

func runProfiles(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gosim-gen profiles", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root containing family definitions")
	check := flags.Bool("check", false, "verify support tables, gates and revisions without writing")
	if code := parseFlags(flags, args, stdout, stderr); code >= 0 {
		return code
	}
	if *root == "" {
		fmt.Fprintln(stderr, "profiles requires a nonempty -root")
		return 2
	}
	if err := profiles.GenerateMetadata(ctx, *root, *check); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runCompare(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gosim-gen compare", flag.ContinueOnError)
	parent := flags.String("parent", "", "immediate parent stable Go family, e.g. go125")
	observation := flags.String("parent-observation", "", "optional complete parent API observation")
	options := inspectionFlags(flags)
	if code := parseFlags(flags, args, stdout, stderr); code >= 0 {
		return code
	}
	if *parent == "" || *options.output == "" {
		fmt.Fprintln(stderr, "compare requires -parent and -output (a new directory)")
		return 2
	}
	arches, err := options.validate()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report, draft, err := audit.Compare(ctx, audit.CompareRequest{
		ParentProfile: *parent, ParentObservation: *observation, Output: *options.output, Arches: arches,
	})
	return finishComparison(ctx, comparisonResult{*options.output, report, draft, err}, stdout, stderr)
}

type comparisonResult struct {
	output string
	report audit.ComparisonReport
	draft  audit.DraftDelta
	err    error
}

func finishComparison(ctx context.Context, result comparisonResult, stdout, stderr io.Writer) int {
	output, report, draft, err := result.output, result.report, result.draft, result.err
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, ctx.Err())
		return 1
	}
	if report.Schema != 0 {
		err = errors.Join(err, audit.WriteComparison(output, report, draft))
		fmt.Fprintf(stdout, "complete=%t api_diff=%s findings=%d proposals=%d\n",
			report.Complete, report.APIDiff, len(report.Findings), len(draft.Proposals))
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !report.Complete {
		return 1
	}
	return 0
}

func runObserve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gosim-gen observe", flag.ContinueOnError)
	profile := flags.String("profile", "", "parent stable Go family to observe at its recorded compiler")
	options := inspectionFlags(flags)
	if code := parseFlags(flags, args, stdout, stderr); code >= 0 {
		return code
	}
	if *profile == "" || *options.output == "" {
		fmt.Fprintln(stderr, "observe requires -profile and -output (a new file)")
		return 2
	}
	arches, err := options.validate()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	snapshot, err := audit.Observe(ctx, audit.ObserveRequest{
		Profile: *profile, Output: *options.output, Arches: arches,
	})
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = audit.WriteObservation(*options.output, snapshot)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "complete=%t contexts=%d records=%d\n",
		snapshot.Complete, len(snapshot.Observations.Contexts), len(snapshot.Observations.Records))
	return 0
}

type inspectionOptions struct{ arches, toolchain, output *string }

func inspectionFlags(flags *flag.FlagSet) inspectionOptions {
	return inspectionOptions{
		arches:    flags.String("arches", "amd64,arm64", "comma-separated simulated Linux source architectures"),
		toolchain: flags.String("toolchain", "", "exact compiler assertion; must match the running compiler"),
		output:    flags.String("output", "", "required new output destination; existing destinations are refused"),
	}
}

func (o inspectionOptions) validate() ([]string, error) {
	if *o.toolchain != "" && *o.toolchain != runtime.Version() {
		return nil, fmt.Errorf("-toolchain must equal running compiler %s", runtime.Version())
	}
	arches := strings.Split(*o.arches, ",")
	seen := map[string]bool{}
	for _, arch := range arches {
		if arch != "amd64" && arch != "arm64" || seen[arch] {
			return nil, fmt.Errorf("invalid or duplicate architecture %q", arch)
		}
		seen[arch] = true
	}
	return arches, nil
}
