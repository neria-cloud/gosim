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
		{"./examples", []string{"rt", "fs", "net"}},
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
