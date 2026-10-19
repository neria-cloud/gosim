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
	switch {
	case pkg.Name == "main" && strings.HasSuffix(pkg.PkgPath, ".test"):
		// XXX: is this correct?
		return PackageKindTestBinary, strings.TrimSuffix(pkg.PkgPath, ".test")

	case strings.HasSuffix(pkg.Name, "_test"):
		return PackageKindTests, strings.TrimSuffix(pkg.PkgPath, "_test")

	case strings.HasSuffix(pkg.ID, ".test]"):
		return PackageKindForTest, pkg.PkgPath

	case !strings.HasSuffix(pkg.Name, "_test") && pkg.PkgPath == pkg.ID:
		return PackageKindBase, pkg.PkgPath

	default:
		panic(fmt.Errorf("unclassified package %s %s %s", pkg.Name, pkg.PkgPath, pkg.ID))
	}
}
