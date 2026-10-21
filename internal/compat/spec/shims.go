package spec

import (
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// VerifyShimBindings checks the version-owned implementation side of a shim
// manifest. VerifyHooks separately checks its upstream side in the pinned GOROOT.
// Native reflect selectors are an intentional, private implementation detail of
// the reflect bridge; exported shim signatures still use the wrapped types.
func VerifyShimBindings(root string, r ResolvedFamily, source SourceContext) error {
	ctx, err := explicitSourceContext(r.Family, source)
	if err != nil {
		return err
	}
	if err := validateDeclarationOverlap(r.Declarations); err != nil {
		return err
	}
	for _, d := range r.Declarations {
		if _, err := declarationRole(d.Hook); err != nil {
			legacy := d.Hook.Context == "" && r.ProfileID != "" && strings.HasPrefix(d.Hook.Target, module+"/internal/compat/"+r.ProfileID+"/shims/")
			if !legacy {
				return err
			}
		}
	}
	return verifyShimBindings(root, r.ShimManifest(source.Arch), ctx)
}

func verifyShimBindings(root string, shims []Hook, ctx build.Context) error {
	if len(shims) == 0 {
		return fmt.Errorf("empty shim manifest")
	}
	packages := make(map[string]map[string]string)
	for _, shim := range shims {
		prefix := module + "/"
		if !strings.HasPrefix(shim.Target, prefix) {
			return fmt.Errorf("invalid shim target %q", shim.Target)
		}
		pkg, symbol, ok := strings.Cut(strings.TrimPrefix(shim.Target, prefix), ".")
		if !ok || ClassifyCompatPackage(module+"/"+pkg) != CompatShims || strings.Contains(pkg, "..") {
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
				selected, err := selectedSourceFile(ctx, filepath.Dir(file), filepath.Base(file))
				if err != nil {
					return err
				}
				if !selected {
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

// An explicit context keeps the maintainer compiler's release/experiment tags
// from selecting a different implementation than the requested source family.
func explicitSourceContext(family string, source SourceContext) (build.Context, error) {
	minor, err := familyMinor(family)
	if err != nil {
		return build.Context{}, err
	}
	if source.Arch != "amd64" && source.Arch != "arm64" || source.OS != "linux" && source.OS != "darwin" || !source.Native && source.OS != "linux" {
		return build.Context{}, fmt.Errorf("unsupported source context %+v", source)
	}
	ctx := build.Context{GOOS: source.OS, GOARCH: source.Arch, Compiler: "gc", CgoEnabled: source.Native && source.Race}
	tags := buildspec.TranslatedSourceTags(source.Race)
	if source.Native {
		tags = "linkname"
		if source.Race {
			tags += ",race"
		}
	}
	ctx.BuildTags = strings.Split(tags, ",")
	for v := 1; v <= minor; v++ {
		ctx.ReleaseTags = append(ctx.ReleaseTags, fmt.Sprintf("go1.%d", v))
	}
	return ctx, nil
}

// MatchFile handles names and constraints but not imports of C. Go listing
// excludes those files when cgo is disabled, so binding selection must too.
func selectedSourceFile(ctx build.Context, dir, name string) (bool, error) {
	selected, err := ctx.MatchFile(dir, name)
	if err != nil || !selected || ctx.CgoEnabled {
		return selected, err
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
	if err != nil {
		return false, err
	}
	for _, imp := range file.Imports {
		if imp.Path.Value == `"C"` || imp.Path.Value == "`C`" {
			return false, nil
		}
	}
	return true, nil
}
