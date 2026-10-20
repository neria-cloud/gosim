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

const (
	runtimeFeature    = "rt"
	filesystemFeature = "fs"
	networkFeature    = "net"
)

func Registry() []Suite {
	return []Suite{
		{"./internal/tests/rt", []string{runtimeFeature}},
		{"./internal/tests/fs", []string{runtimeFeature, filesystemFeature}},
		{"./internal/tests/net", []string{runtimeFeature, networkFeature}},
		{"./internal/tests/integration", []string{runtimeFeature, filesystemFeature, networkFeature}},
		{"./nemesis", []string{runtimeFeature, filesystemFeature, networkFeature}},
		{"./examples/simple", []string{runtimeFeature}},
		{"./examples/bolt", []string{runtimeFeature, filesystemFeature}},
		{"./examples/network/machines", []string{runtimeFeature, networkFeature}},
		{"./examples/network/latency", []string{runtimeFeature, networkFeature}},
	}
}

func Packages(names []string) ([]string, error) {
	selectedNames, err := buildspec.Features(names)
	if err != nil {
		return nil, err
	}
	var packages []string
	for _, suite := range Registry() {
		if slices.ContainsFunc(suite.Requires, func(name string) bool {
			return !slices.Contains(selectedNames, name)
		}) {
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
	selectedNames, err := buildspec.Features(names)
	if err != nil {
		return nil, err
	}
	packages := []string{"./rt/runtime/...", "./internal/hostabi", "./internal/contracts"}
	if slices.Contains(selectedNames, filesystemFeature) {
		packages = append(packages, "./internal/fs/model")
	}
	if slices.Contains(selectedNames, networkFeature) {
		packages = append(packages, "./internal/net/model")
	}
	return packages, nil
}
