package spec

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// VerifyShimBindings checks the version-owned implementation side of a shim
// manifest. VerifyHooks separately checks its upstream side in the pinned GOROOT.
// Native reflect selectors are an intentional, private implementation detail of
// the reflect bridge; exported shim signatures still use the wrapped types.
func VerifyShimBindings(root string, shims []Hook) error {
	if len(shims) == 0 {
		return errors.New("empty shim manifest")
	}
	packages := make(map[string]map[string]string)
	for _, shim := range shims {
		prefix := module + "/"
		if !strings.HasPrefix(shim.Target, prefix) {
			return fmt.Errorf("invalid shim target %q", shim.Target)
		}
		pkg, symbol, ok := strings.Cut(strings.TrimPrefix(shim.Target, prefix), ".")
		if !ok || !isFamilyShim(pkg) || strings.Contains(pkg, "..") {
			return fmt.Errorf("invalid shim target %q", shim.Target)
		}
		declarations, ok := packages[pkg]
		if !ok {
			declarations = make(map[string]string)
			files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pkg), "*.go"))
			if err != nil {
				return err
			}
			for _, file := range files {
				if strings.HasSuffix(file, "_test.go") {
					continue
				}
				if err := readHookDeclarations(file, pkg, declarations); err != nil {
					return err
				}
			}
			packages[pkg] = declarations
		}
		got, ok := declarations[pkg+"."+symbol]
		if shim.Package == "reflect" {
			got = strings.ReplaceAll(got, "reflect.", "")
		}
		if !ok || got != shim.Signature {
			return fmt.Errorf("shim ABI drift for %s: got %q, expected %q", shim.Target, got, shim.Signature)
		}
	}
	return nil
}

func isFamilyShim(pkg string) bool {
	for _, family := range buildspec.SupportedFamilies() {
		if strings.HasPrefix(pkg, "internal/compat/"+family.Profile+"/shims/") {
			return true
		}
	}
	return false
}
