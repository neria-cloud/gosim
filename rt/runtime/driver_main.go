package gosimruntime

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
)

// TODO(migration): rename to TestMain and remove the legacy Runtime entry in Task 22.
// TestMainDriver runs each request directly through the generated Driver. It
// never enters the legacy scheduler before calling Driver.Run.
func TestMainDriver(driver Driver) {
	trace := flag.String("simtrace", "", "set of comma-separated traces to enable")
	seedsFlag := flag.String("seeds", "1", "comma-separated list of seeds and ranges")
	jsonLog := flag.String("jsonlogout", "", "path for captured JSON logs")
	flag.Parse()
	fatal := func(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	execute := func(config RunConfig, skip string) RunResult {
		return driver.Run(config, func() {
			test, ok := allTests[config.Test]
			if !ok {
				SetAbortError(ErrTestFailed)
				return
			}
			if !driver.TestEntrypoint(config.Test, skip, []Test{{Name: config.Test, Test: test}}) {
				SetAbortError(ErrTestFailed)
			}
		})
	}
	if *metatest {
		// TODO(migration): Task 23 adds the identity/protocol-2 handshake.
		input, output := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
		for {
			var request RunConfig
			if err := input.Decode(&request); err != nil {
				if err == io.EOF {
					return
				}
				fatal(err)
				return
			}
			var response any
			if request.Test == "listtests" {
				response = slices.Sorted(maps.Keys(allTests))
			} else {
				response = execute(request, "")
			}
			if err := output.Encode(response); err != nil {
				fatal(err)
				return
			}
		}
	}
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "test.") && !supportedFlags[f.Name] {
			fatal(fmt.Errorf("flag %s is not supported by gosim", f.Name))
		}
	})
	match := flag.Lookup("test.run").Value.(flag.Getter).Get().(string)
	skip := flag.Lookup("test.skip").Value.(flag.Getter).Get().(string)
	seeds, err := parseSeeds(*seedsFlag)
	if err != nil {
		fatal(err)
		return
	}
	var logs *os.File
	if *jsonLog != "" {
		logs, err = os.Create(*jsonLog)
		if err != nil {
			fatal(err)
			return
		}
	}
	failed := false
	for _, span := range seeds {
		for seed := span.start; seed <= span.end; seed++ {
			for _, test := range allTestsSlice {
				config := RunConfig{Test: test.Name, Seed: seed, Simtrace: *trace}
				result := driver.Run(config, func() {
					if !driver.TestEntrypoint(match, skip, []Test{test}) {
						SetAbortError(ErrTestFailed)
					}
				})
				if len(result.LogOutput) > 0 {
					writer := makeConsoleLogger(os.Stderr)
					for _, line := range bytes.SplitAfter(result.LogOutput, []byte("\n")) {
						if len(line) == 0 {
							continue
						}
						if _, err := writer.Write(line); err != nil {
							fatal(err)
							return
						}
					}
				}
				if logs != nil {
					if _, err := logs.Write(result.LogOutput); err != nil {
						fatal(err)
						return
					}
				}
				if result.Failed {
					failed = true
					if result.Err != "" {
						fmt.Fprintln(os.Stderr, result.Err)
					}
				}
			}
			if seed == span.end {
				break
			} // avoid signed overflow at the last seed
		}
	}
	if logs != nil {
		if err := logs.Close(); err != nil {
			fatal(err)
			return
		}
	}
	if failed {
		os.Exit(1)
	}
	os.Exit(0)
}
