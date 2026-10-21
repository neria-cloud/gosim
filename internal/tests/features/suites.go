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
	Native   bool // Has native tests in addition to translated coverage.
}

func Registry() []Suite {
	return []Suite{
		{"./internal/tests/rt", []string{"rt"}, true},
		{"./internal/tests/fs", []string{"rt", "fs"}, true},
		{"./internal/tests/net", []string{"rt", "net"}, true},
		{"./internal/tests/integration", []string{"rt", "fs", "net"}, true},
		{"./nemesis", []string{"rt", "fs", "net"}, true},
		{"./examples/simple", []string{"rt"}, true},
		{"./examples/bolt", []string{"rt", "fs"}, false},
		{"./examples/network/machines", []string{"rt", "net"}, false},
		{"./examples/network/latency", []string{"rt", "net"}, false},
		{"./examples/etcd", []string{"rt", "fs", "net"}, false},
	}
}

func Packages(names []string) ([]string, error) {
	return applicablePackages(names, false)
}

// MetatestPackages excludes sim-only packages from native go test invocations.
// Those packages retain their build and execution coverage through Packages.
func MetatestPackages(names []string) ([]string, error) {
	return applicablePackages(names, true)
}

func applicablePackages(names []string, nativeOnly bool) ([]string, error) {
	names, err := buildspec.Features(names)
	if err != nil {
		return nil, err
	}
	var packages []string
	for _, suite := range Registry() {
		if nativeOnly && !suite.Native {
			continue
		}
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
