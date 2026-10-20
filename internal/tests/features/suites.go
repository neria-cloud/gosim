// Package features records complete package applicability for candidate testing.
// Selection happens before package loading, so disabled provider imports and init
// functions cannot hide behind a test-name filter.
package features

import (
	"slices"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type Suite struct {
	Package  string
	Requires []string
}

func Registry() []Suite {
	return []Suite{
		{"./internal/tests/rt", []string{"rt"}},
		{"./internal/tests/fs", []string{"rt", "fs"}},
		{"./internal/tests/net", []string{"rt", "net"}},
		{"./internal/tests/integration", []string{"rt", "fs", "net"}},
		{"./nemesis", []string{"rt", "fs", "net"}},
		{"./examples/simple", []string{"rt"}},
		{"./examples/bolt", []string{"rt", "fs"}},
		{"./examples/network/machines", []string{"rt", "net"}},
		{"./examples/network/latency", []string{"rt", "net"}},
	}
}

func Packages(names []string) ([]string, error) {
	names, err := buildspec.Features(names)
	if err != nil {
		return nil, err
	}
	var packages []string
	for _, suite := range Registry() {
		if slices.ContainsFunc(suite.Requires, func(name string) bool { return !slices.Contains(names, name) }) {
			continue
		}
		packages = append(packages, suite.Package)
	}
	return packages, nil
}

// NativePackages covers runtime and model units without pulling the other
// provider into reduced-feature jobs through provider integration test imports.
// Full-feature certification runs ./..., including those integration tests.
func NativePackages(names []string) ([]string, error) {
	names, err := buildspec.Features(names)
	if err != nil {
		return nil, err
	}
	packages := []string{"./rt/runtime/...", "./internal/hostabi", "./internal/contracts"}
	if slices.Contains(names, "fs") {
		packages = append(packages, "./internal/fs/model")
	}
	if slices.Contains(names, "net") {
		packages = append(packages, "./internal/net/model")
	}
	return packages, nil
}
