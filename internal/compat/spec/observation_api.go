package spec

import (
	"fmt"
	"go/types"
	"slices"
	"strings"
)

// API scope follows reviewed upstream surfaces, independently of the number of
// declared shim bindings. Policy wildcards are not an upstream API inventory.
func ObservationAPIPackages(r ResolvedFamily) []string {
	var paths []string
	add := func(path string) {
		if path != module && !strings.HasPrefix(path, module+"/") {
			paths = append(paths, path)
		}
	}
	for _, d := range r.Declarations {
		add(d.Hook.Package)
	}
	for _, b := range r.Bridges {
		add(b.Package)
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func (s *sourceInspection) observeAPIs(paths []string, context InspectedContext) ([]Observation, error) {
	var records []Observation
	qualifier := func(p *types.Package) string { return p.Path() }
	add := func(path, symbol, value string) {
		records = append(records, Observation{Kind: "api", Key: []string{path, symbol}, Context: context, Present: true, Value: []byte(observationJSON(value))})
	}
	for _, path := range paths {
		pkg, err := s.Import(path)
		if err != nil {
			return nil, fmt.Errorf("observe API %s: %w", path, err)
		}
		for _, name := range pkg.Scope().Names() {
			obj := pkg.Scope().Lookup(name)
			if !obj.Exported() {
				continue
			}
			value := types.ObjectString(obj, qualifier)
			if c, ok := obj.(*types.Const); ok {
				value += " = " + c.Val().ExactString()
			}
			add(path, name, value)
			if _, ok := obj.(*types.TypeName); !ok {
				continue
			}
			for _, receiver := range []struct {
				prefix string
				typ    types.Type
			}{{"", obj.Type()}, {"*", types.NewPointer(obj.Type())}} {
				methods := types.NewMethodSet(receiver.typ)
				for i := 0; i < methods.Len(); i++ {
					method := methods.At(i)
					if !method.Obj().Exported() {
						continue
					}
					add(path, receiver.prefix+name+"."+method.Obj().Name(), types.TypeString(method.Type(), qualifier))
				}
			}
		}
	}
	return records, nil
}

// Native contexts cover their native contracts; the complete bridge/declaration
// inventory belongs to translated contexts. Both scans record the same union.
func observationAPIContextPackages(r ResolvedFamily, selection SourceContext) []string {
	if !selection.Native {
		return ObservationAPIPackages(r)
	}
	var paths []string
	for _, d := range observationDeclarations(r, selection) {
		if d.Hook.Package != module && !strings.HasPrefix(d.Hook.Package, module+"/") {
			paths = append(paths, d.Hook.Package)
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

// ObservationInventory is the single context-specific inventory used by
// preparation, collection and snapshot validation. API scope remains finite.
func ObservationInventory(r ResolvedFamily, selection SourceContext, includeAPIs bool) (declarations []Declaration, packages, apiPackages []string) {
	declarations = observationDeclarations(r, selection)
	for _, d := range declarations {
		packages = append(packages, d.Hook.Package)
	}
	if !selection.Native {
		for _, b := range r.Bridges {
			if b.Package != module && !strings.HasPrefix(b.Package, module+"/") {
				packages = append(packages, b.Package)
			}
		}
		for _, l := range r.Architectures[selection.Arch].Layouts {
			packages = append(packages, l.Package)
		}
		for _, c := range r.Architectures[selection.Arch].Constants {
			packages = append(packages, c.Package)
		}
	}
	if includeAPIs {
		apiPackages = observationAPIContextPackages(r, selection)
		packages = append(packages, apiPackages...)
	}
	slices.Sort(packages)
	return declarations, slices.Compact(packages), apiPackages
}
