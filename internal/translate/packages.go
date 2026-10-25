package translate

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/gosimtool"
)

const (
	gosimModPath        = gosimtool.Module
	gosimruntimePackage = gosimModPath + "/rt/runtime"
	runtimeABIPackage   = gosimruntimePackage + "/abi"
	loadSyntaxAndTypes  = packages.NeedSyntax | packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles | packages.NeedImports
)

type packageKind string

const (
	PackageKindTestBinary = "testbinary"
	PackageKindBase       = "base"
	PackageKindForTest    = "fortest"
	PackageKindTests      = "tests"
)

func classifyPackage(pkg *packages.Package) (packageKind, string) {
	owner := testOwner(pkg)
	switch {
	case pkg.Name == "main" && strings.HasSuffix(pkg.PkgPath, ".test"):
		// XXX: is this correct?
		return PackageKindTestBinary, strings.TrimSuffix(pkg.PkgPath, ".test")

	case strings.HasSuffix(pkg.Name, "_test"):
		return PackageKindTests, strings.TrimSuffix(pkg.PkgPath, "_test")

	case owner != "" && owner == pkg.PkgPath:
		return PackageKindForTest, pkg.PkgPath

	case owner != "":
		// Go also recompiles ordinary dependencies for a test binary when
		// they import its augmented package. They contain production source,
		// not the package-under-test's extra declarations. Translate them in
		// full, retaining their distinct ID and imports for cache identity.
		return PackageKindBase, pkg.PkgPath

	case !strings.HasSuffix(pkg.Name, "_test") && pkg.PkgPath == pkg.ID:
		return PackageKindBase, pkg.PkgPath

	default:
		panic(fmt.Errorf("unclassified package %s %s %s", pkg.Name, pkg.PkgPath, pkg.ID))
	}
}

// testOwner returns the package whose test binary induced this variant. An
// indirect dependency must be type checked in that context, not as its own test.
func testOwner(pkg *packages.Package) string {
	prefix, suffix, ok := strings.Cut(pkg.ID, " [")
	if !ok || prefix != pkg.PkgPath || !strings.HasSuffix(suffix, ".test]") {
		return ""
	}
	return strings.TrimSuffix(suffix, ".test]")
}
