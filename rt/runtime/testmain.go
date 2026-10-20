package gosimruntime

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

const seedDecimalBase = 10

type Test struct {
	Name string
	Test any // func()
}

var (
	allTests      map[string]any
	allTestsSlice []Test
)

func SetAllTests(tests []Test) {
	allTests = make(map[string]any)
	allTestsSlice = tests
	for _, test := range tests {
		allTests[test.Name] = test.Test
	}
}

var metatest = flag.Bool("metatest", false, "")

// Copied in metatesting.RunConfig. Keep in sync.
type RunConfig struct {
	Test     string
	Seed     int64
	ExtraEnv []string
	Simtrace string
}

// Copied in metatesting.RunResult. Keep in sync.
type RunResult struct {
	Seed           int64
	Checksum       []byte
	Failed         bool
	LogOutput      []byte
	Err            string // TODO: reconsider this type?
	BuildDigest    string
	ReplayRevision string
}

type Driver interface {
	Run(config RunConfig, main func()) RunResult
	TestEntrypoint(match, skip string, tests []Test) bool
}

// MetatestHandshake binds the request stream to one generated binary.
type MetatestHandshake struct {
	Protocol       int    `json:"protocol"`
	IdentityDigest string `json:"identity_digest"`
	Toolchain      string `json:"toolchain"`
}

// subset of flags defined by the testing package that we parse
// and support.
var supportedFlags = map[string]bool{
	// supported:
	"test.run":  true,
	"test.skip": true,

	// not supported but (hopefully) harmless to ignore:
	"test.paniconexit0": true,
	"test.testlogfile":  true,
	"test.timeout":      true,
	"test.v":            true,
}

type seedRange struct {
	start, end int64
}

func parseSeeds(seedsStr string) ([]seedRange, error) {
	var seeds []seedRange
	for _, rangeStr := range strings.Split(seedsStr, ",") {
		startStr, endStr, ok := strings.Cut(rangeStr, "-")
		if !ok {
			endStr = startStr
		}

		start, err := strconv.ParseInt(startStr, seedDecimalBase, 64)
		if err != nil {
			return nil, fmt.Errorf("bad start of seed range %q: %v", startStr, err)
		}
		end, err := strconv.ParseInt(endStr, seedDecimalBase, 64)
		if err != nil {
			return nil, fmt.Errorf("bad end of seed range %q: %v", endStr, err)
		}

		seeds = append(seeds, seedRange{start: start, end: end})
	}
	return seeds, nil
}
