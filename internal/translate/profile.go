package translate

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

// PackagePolicies owns a validated snapshot. Maps are private so a caller cannot
// turn an unreviewed package into a native island after selection validation.
type PackagePolicies struct {
	profile            spec.Profile
	manifest           spec.PolicyManifest
	features           []string
	compositionPackage string
	hooks              map[PackageSelector]PackageSelector
	replacements       map[PackageSelector]PackageSelector
	packages           map[string]string
	native             map[string]bool
	shared             map[PackageSelector]bool
	definitions        map[PackageSelector]bool // pushed definitions in the selected translated source graph
}

func Policies(selection buildspec.Selection) (PackagePolicies, error) {
	p, m, err := selected.Resolve(selection)
	if err != nil {
		return PackagePolicies{}, err
	}
	digest, err := buildspec.Digest(selection.Identity)
	if err != nil {
		return PackagePolicies{}, err
	}
	result := PackagePolicies{
		profile: p, manifest: m, features: slices.Clone(selection.Identity.Features), compositionPackage: gosimtool.Module + "/.gosim/v2/" + digest + "/input/gosimcomposition",
		hooks: make(map[PackageSelector]PackageSelector), replacements: make(map[PackageSelector]PackageSelector),
		packages: make(map[string]string), native: make(map[string]bool), shared: make(map[PackageSelector]bool),
	}
	for _, h := range p.Hooks {
		target, err := splitSymbol(h.Target)
		if err != nil {
			return PackagePolicies{}, err
		}
		result.hooks[PackageSelector{Pkg: h.Package, Selector: h.Symbol}] = target
	}
	for _, b := range m.Bridges {
		if b.Replacement != "" {
			result.packages[b.Package] = b.Replacement
		} else {
			policy, err := p.Policy(b.Package, true)
			if err != nil {
				return PackagePolicies{}, err
			}
			if policy.Mode == "bridge" {
				result.native[b.Package] = true
			}
		}
		for _, r := range b.Rewrites {
			result.replacements[PackageSelector{Pkg: b.Package, Selector: r.Symbol}] = PackageSelector{Pkg: r.Package, Selector: r.Target}
		}
	}
	for _, p := range p.Packages {
		if p.Mode == "native-pure" {
			result.native[p.Path] = true
		}
	}
	for _, g := range m.SharedGlobals {
		result.shared[PackageSelector{Pkg: g.Package, Selector: g.Symbol}] = true
	}
	return result, nil
}

func splitSymbol(symbol string) (PackageSelector, error) {
	start := strings.LastIndex(symbol, "/") + 1
	dot := strings.IndexByte(symbol[start:], '.')
	if dot < 1 || start+dot+1 == len(symbol) {
		return PackageSelector{}, fmt.Errorf("invalid link target %q", symbol)
	}
	return PackageSelector{Pkg: symbol[:start+dot], Selector: symbol[start+dot+1:]}, nil
}

func standardPackage(pkg *packages.Package) bool {
	return pkg.Module == nil && !strings.Contains(strings.Split(pkg.PkgPath, "/")[0], ".")
}

func (p PackagePolicies) packagePolicy(path string, standard bool) (spec.PackagePolicy, error) {
	if p.profile.ID == "" {
		return spec.PackagePolicy{}, fmt.Errorf("package policies are unresolved")
	}
	prefix := gosimtool.Module + "/internal/"
	for _, retired := range []string{"simulation", "hooks", "reflect", "testing", "maps_shim", "synctest"} {
		if path == prefix+retired || strings.HasPrefix(path, prefix+retired+"/") {
			return spec.PackagePolicy{}, fmt.Errorf("retired application package %s", path)
		}
	}
	if strings.HasPrefix(path, prefix+"compat/") && !strings.HasPrefix(path, prefix+"compat/"+p.profile.ID+"/hooks/") && !strings.HasPrefix(path, prefix+"compat/"+p.profile.ID+"/shims/") {
		return spec.PackagePolicy{}, fmt.Errorf("nonselected or host-only compatibility package %s", path)
	}
	for _, feature := range []string{"fs", "net"} {
		if (path == prefix+feature || strings.HasPrefix(path, prefix+feature+"/")) && !slices.Contains(p.features, feature) {
			return spec.PackagePolicy{}, fmt.Errorf("disabled provider import %s", path)
		}
	}
	return p.profile.Policy(path, standard)
}

func (p PackagePolicies) importPolicy(from, path string, standard, native bool) error {
	policy, err := p.packagePolicy(path, standard)
	if err != nil {
		return err
	}
	if native {
		return p.manifest.AllowNativeImport(p.profile, from, path, standard)
	}
	// A concrete provider is constructed only by generated composition. Provider
	// implementation packages may import their own model, never the other domain.
	for _, feature := range []string{"fs", "net"} {
		provider := gosimtool.Module + "/internal/" + feature
		if path == provider || strings.HasPrefix(path, provider+"/") {
			if from != provider && !strings.HasPrefix(from, provider+"/") && from != p.compositionPackage {
				return fmt.Errorf("concrete provider %s imported outside composition by %s", path, from)
			}
		}
	}
	// Bridge admission is symbol-specific below. Blank imports of a replacement
	// load the replacement, not the original package's host initialization.
	if policy.Mode == "bridge" {
		_, err = p.manifest.ResolveSymbol(p.profile, path, "", standard)
	}
	return err
}

func nativeDirective(imp *ast.ImportSpec) bool {
	for _, group := range []*ast.CommentGroup{imp.Doc, imp.Comment} {
		if group != nil {
			for _, c := range group.List {
				if strings.TrimSpace(c.Text) == "//gosim:notranslate" {
					return true
				}
			}
		}
	}
	return false
}

// ValidateBoundary checks a typed source package before transformations. It
// deliberately does not traverse native dependencies: only the two explicit
// engine/ABI islands and reviewed pure/bridge symbols can cross this boundary.
func ValidateBoundary(pkg *packages.Package, policies PackagePolicies) error {
	if pkg == nil {
		return fmt.Errorf("nil package at translation boundary")
	}
	policy, err := policies.packagePolicy(pkg.PkgPath, standardPackage(pkg))
	if err != nil {
		return err
	}
	if policy.Mode != "translate" {
		return nil
	}
	if pkg.TypesInfo == nil || pkg.Types == nil || len(pkg.Syntax) == 0 && len(pkg.CompiledGoFiles) != 0 {
		return fmt.Errorf("missing typed source for %s", pkg.PkgPath)
	}
	if len(pkg.Errors) > 0 {
		return fmt.Errorf("invalid source for %s: %s", pkg.PkgPath, pkg.Errors[0])
	}
	var used map[string]bool
	if len(pkg.CompiledGoFiles) != 0 {
		used, err = survivingImports(pkg, policies)
		if err != nil {
			return err
		}
	}
	for _, file := range pkg.Syntax {
		natives := make(map[*types.PkgName]bool)
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if used != nil && !used[path] {
				continue
			}
			dep := pkg.Imports[path]
			if dep == nil {
				return fmt.Errorf("unresolved import %s from %s", path, pkg.PkgPath)
			}
			native := nativeDirective(imp)
			if err := policies.importPolicy(pkg.PkgPath, dep.PkgPath, standardPackage(dep), native); err != nil {
				return fmt.Errorf("%s imports %s: %w", pkg.PkgPath, path, err)
			}
			if native {
				var name *types.PkgName
				if imp.Name != nil {
					name, _ = pkg.TypesInfo.Defs[imp.Name].(*types.PkgName)
				} else {
					name, _ = pkg.TypesInfo.Implicits[imp].(*types.PkgName)
				}
				natives[name] = true
			}
		}
		var failure error
		ast.Inspect(file, func(n ast.Node) bool {
			if failure != nil {
				return false
			}
			if fn, ok := n.(*ast.FuncDecl); ok {
				if _, hooked := policies.hooks[PackageSelector{Pkg: pkg.PkgPath, Selector: fn.Name.Name}]; hooked {
					return false
				}
			}
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := pkg.TypesInfo.Uses[ident]
			if obj == nil || obj.Pkg() == nil || obj.Pkg() == pkg.Types {
				return true
			}
			if _, isPackage := obj.(*types.PkgName); isPackage {
				return true
			}
			if field, ok := obj.(*types.Var); ok && field.IsField() {
				return true
			}
			path := obj.Pkg().Path()
			for imported := range natives {
				if imported != nil && imported.Imported().Path() == path {
					return true
				}
			}
			dep := pkg.Imports[path]
			// Methods of transitive native types are checked by their actual owner.
			// Ordinary transitive translated types stay in the same type universe.
			standard := !strings.Contains(strings.Split(path, "/")[0], ".")
			if dep != nil {
				standard = standardPackage(dep)
			}
			symbol := obj.Name()
			if fn, ok := obj.(*types.Func); ok {
				sig := fn.Type().(*types.Signature)
				if sig.Recv() != nil {
					recv := sig.Recv().Type()
					prefix := ""
					if ptr, ok := recv.(*types.Pointer); ok {
						prefix = "*"
						recv = ptr.Elem()
					}
					if named, ok := recv.(*types.Named); ok {
						symbol = prefix + named.Obj().Name() + "." + symbol
					}
				}
			}
			if standardPackage(pkg) {
				if _, hooked := policies.hooks[PackageSelector{Pkg: path, Selector: symbol}]; hooked {
					return true
				}
				if _, rewritten := policies.replacements[PackageSelector{Pkg: path, Selector: symbol}]; rewritten {
					return true
				}
			}
			_, failure = policies.manifest.ResolveSymbol(policies.profile, path, symbol, standard)
			return failure == nil
		})
		if failure != nil {
			return fmt.Errorf("%s: %w", pkg.PkgPath, failure)
		}
		if err := policies.validateDeclarations(pkg, file); err != nil {
			return err
		}
	}
	return nil
}

func (p PackagePolicies) validateDeclarations(pkg *packages.Package, file *ast.File) error {
	linked := make(map[string]bool)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			fields := strings.Fields(comment.Text)
			if len(fields) == 0 || fields[0] != "//go:linkname" {
				continue
			}
			if len(fields) != 2 && len(fields) != 3 {
				return fmt.Errorf("malformed linkname in %s", pkg.PkgPath)
			}
			local := PackageSelector{Pkg: pkg.PkgPath, Selector: fields[1]}
			if _, ok := p.hooks[local]; ok {
				linked[fields[1]] = true
				continue
			}
			if len(fields) == 2 {
				continue
			} // an exported local body is checked below
			target, err := splitSymbol(fields[2])
			if err != nil {
				return err
			}
			if runtimeRandomLink(target) || runtimeAuxvLink(target) {
				obj, _ := pkg.Types.Scope().Lookup(fields[1]).(*types.Func)
				if obj == nil || !validRuntimeLinkSignature(target, obj.Type().(*types.Signature)) {
					return fmt.Errorf("invalid runtime link signature: %s", fields[1])
				}
				if _, ok := p.replacements[target]; !ok {
					return fmt.Errorf("missing modeled runtime link adapter")
				}
				linked[fields[1]] = true
				continue
			}
			standard := !strings.Contains(strings.Split(target.Pkg, "/")[0], ".")
			if imported := pkg.Imports[target.Pkg]; imported != nil {
				standard = standardPackage(imported)
			}
			targetPolicy, err := p.packagePolicy(target.Pkg, standard)
			if err != nil || targetPolicy.Mode != "translate" {
				return fmt.Errorf("unreviewed native linkname %s.%s -> %s", pkg.PkgPath, fields[1], fields[2])
			}
			if standardPackage(pkg) {
				if _, hooked := p.hooks[target]; hooked {
					linked[fields[1]] = true
					continue
				}
			}
			// Private application directives cannot bypass symbol-level rejections.
			if _, err := p.manifest.ResolveSymbol(p.profile, target.Pkg, target.Selector, false); err != nil {
				return err
			}
			linked[fields[1]] = true
		}
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body != nil {
			continue
		}
		if _, ok := p.hooks[PackageSelector{Pkg: pkg.PkgPath, Selector: fn.Name.Name}]; !ok && !linked[fn.Name.Name] {
			if p.definitions[PackageSelector{Pkg: pkg.PkgPath, Selector: fn.Name.Name}] {
				continue
			}
			return fmt.Errorf("unreviewed bodyless function %s.%s", pkg.PkgPath, fn.Name.Name)
		}
	}
	return nil
}

// These legacy symbols are stable across the selected families. Validate their
// primitive signatures before redirecting private application linknames.
func runtimeRandomLink(target PackageSelector) bool {
	return target.Pkg == "runtime" && (target.Selector == "fastrand" || target.Selector == "fastrandn" || target.Selector == "fastrand64")
}

func validRuntimeRandomSignature(target PackageSelector, sig *types.Signature) bool {
	if sig.Variadic() || sig.Recv() != nil || sig.Results().Len() != 1 {
		return false
	}
	result := types.Typ[types.Uint32]
	if target.Selector == "fastrand64" {
		result = types.Typ[types.Uint64]
	}
	if !types.Identical(sig.Results().At(0).Type(), result) {
		return false
	}
	if target.Selector == "fastrandn" {
		return sig.Params().Len() == 1 && types.Identical(sig.Params().At(0).Type(), types.Typ[types.Uint32])
	}
	return sig.Params().Len() == 0
}

func runtimeAuxvLink(target PackageSelector) bool {
	return target.Pkg == "runtime" && target.Selector == "getAuxv"
}

func validRuntimeLinkSignature(target PackageSelector, sig *types.Signature) bool {
	if runtimeAuxvLink(target) {
		return !sig.Variadic() && sig.Recv() == nil && sig.Params().Len() == 0 && sig.Results().Len() == 1 && types.Identical(sig.Results().At(0).Type(), types.NewSlice(types.Typ[types.Uintptr]))
	}
	return validRuntimeRandomSignature(target, sig)
}
