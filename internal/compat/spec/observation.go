package spec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type InspectedContext struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Race   bool   `json:"race"`
	Native bool   `json:"native"`
}
type CompilerRecord struct {
	Version      string                 `json:"version"`
	HostOS       string                 `json:"host_os"`
	HostArch     string                 `json:"host_arch"`
	Fingerprints buildspec.Fingerprints `json:"fingerprints"`
}
type SourceLocation struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}
type ContextResult struct {
	Context  InspectedContext `json:"context"`
	Complete bool             `json:"complete"`
	Errors   []string         `json:"errors,omitempty"`
}
type Observation struct {
	Kind      string           `json:"kind"`
	Key       []string         `json:"key"`
	Context   InspectedContext `json:"context"`
	Present   bool             `json:"present"`
	Value     json.RawMessage  `json:"value,omitempty"`
	Locations []SourceLocation `json:"locations,omitempty"`
}
type ObservationSet struct {
	APIPackages []string        `json:"api_packages"`
	Contexts    []ContextResult `json:"contexts"`
	Records     []Observation   `json:"records"`
}
type ObservationRequest struct {
	Family                 ResolvedFamily
	Origin                 Origin
	Contexts               []SourceContext
	IncludeAPIs            bool
	IncludeStdlibInventory bool
}

// CollectSourceObservations receives an already prepared private module. It
// independently checks compiler identity; callers cannot inject command policies.
func CollectSourceObservations(ctx context.Context, tc buildspec.Toolchain, dir string, req ObservationRequest) (ObservationSet, error) {
	result := ObservationSet{APIPackages: []string{}, Contexts: []ContextResult{}, Records: []Observation{}}
	if err := validateObservationCompiler(ctx, tc); err != nil {
		return result, err
	}
	if len(req.Contexts) == 0 {
		return result, fmt.Errorf("no observation contexts requested")
	}
	if req.IncludeAPIs {
		result.APIPackages = ObservationAPIPackages(req.Family)
	}
	seen := map[SourceContext]bool{}
	var failures []error
	for _, selection := range req.Contexts {
		if seen[selection] {
			return result, fmt.Errorf("duplicate observation context %+v", selection)
		}
		seen[selection] = true
		status := ContextResult{Context: InspectedContext(selection)}
		records, err := collectContext(ctx, tc, dir, req, selection)
		if err != nil {
			status.Errors = []string{err.Error()}
			failures = append(failures, err)
		} else {
			status.Complete = true
			result.Records = append(result.Records, records...)
		}
		result.Contexts = append(result.Contexts, status)
	}
	slices.SortFunc(result.Contexts, func(a, b ContextResult) int {
		return strings.Compare(observationJSON(a.Context), observationJSON(b.Context))
	})
	slices.SortFunc(result.Records, func(a, b Observation) int { return strings.Compare(observationJSON(a), observationJSON(b)) })
	return result, errors.Join(failures...)
}

func validateObservationCompiler(ctx context.Context, tc buildspec.Toolchain) error {
	if _, err := buildspec.ParseGoVersion(tc.Version); err != nil {
		return err
	}
	if runtime.Compiler != "gc" || tc.Version != runtime.Version() || tc.HostOS != runtime.GOOS || tc.HostArch != runtime.GOARCH {
		return fmt.Errorf("observation compiler must match running %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if (setting.Key == "GOEXPERIMENT" && setting.Value != "") || (setting.Key == "-compiler" && setting.Value != "gc") || (setting.Key == "-tags" && setting.Value != "" && setting.Value != "linkname") {
				return fmt.Errorf("inspector built with unapproved %s=%q", setting.Key, setting.Value)
			}
		}
	}
	root, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		return err
	}
	if tc.GOROOT != root {
		return fmt.Errorf("observation GOROOT differs from running installation")
	}
	out, err := buildspec.SourceInspectionCommand(ctx, tc, "", "version").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != "go version "+tc.Version+" "+tc.HostOS+"/"+tc.HostArch {
		return fmt.Errorf("observation executable identity mismatch")
	}
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return err
	}
	if strings.SplitN(string(version), "\n", 2)[0] != tc.Version {
		return fmt.Errorf("observation installation VERSION mismatch")
	}
	return nil
}

func observationJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func observationDeclarations(r ResolvedFamily, selection SourceContext) []Declaration {
	var result []Declaration
	for _, d := range r.Declarations {
		if declarationApplies(d, selection.Arch) && ((!selection.Native && d.Hook.Context == "") || (selection.Native && (d.Hook.Context == "host" || selection.Race && d.Hook.Context == "host-race"))) {
			result = append(result, d)
		}
	}
	return result
}

func collectContext(ctx context.Context, tc buildspec.Toolchain, dir string, req ObservationRequest, selection SourceContext) ([]Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if selection.Arch != "amd64" && selection.Arch != "arm64" || !selection.Native && selection.OS != "linux" || selection.Native && (selection.OS != tc.HostOS || selection.Arch != tc.HostArch) {
		return nil, fmt.Errorf("invalid source context %+v", selection)
	}
	decls, paths, apiPaths := ObservationInventory(req.Family, selection, req.IncludeAPIs)
	arch := req.Family.Architectures[selection.Arch]
	for _, path := range paths {
		if !validContractPackage(path) {
			return nil, fmt.Errorf("invalid observation package %q", path)
		}
	}
	query := append(slices.Clone(paths), "std")
	if len(query) == 0 {
		return nil, fmt.Errorf("no reviewed packages for context %+v", selection)
	}
	sources, err := inspectSourcesForObservation(ctx, tc, dir, selection, true, query)
	if err != nil {
		return nil, err
	}
	var records []Observation
	add := func(kind string, key []string, present bool, value any, locations ...SourceLocation) {
		o := Observation{Kind: kind, Key: key, Context: InspectedContext(selection), Present: present, Locations: locations}
		if present {
			o.Value = json.RawMessage(observationJSON(value))
		}
		records = append(records, o)
	}
	symbols := map[string]string{}
	locations := map[string]SourceLocation{}
	files := map[string]string{}
	for _, file := range req.Origin.Files {
		if !fs.ValidPath(file.Path) || !strings.HasPrefix(file.Path, "src/") || strings.Contains(file.Path, "\\") {
			return nil, fmt.Errorf("invalid tracked source %q", file.Path)
		}
		files[file.Path] = filepath.Join(tc.GOROOT, filepath.FromSlash(file.Path))
	}
	for _, path := range paths {
		pkg, present := sources.packages[path]
		add("package", []string{path}, present, path)
		if !present {
			continue
		}
		for _, name := range append(slices.Clone(pkg.GoFiles), pkg.CgoFiles...) {
			filename := filepath.Join(pkg.Dir, name)
			rel, _ := filepath.Rel(tc.GOROOT, filename)
			rel = filepath.ToSlash(rel)
			if !strings.HasPrefix(rel, "src/") {
				rel = "module/" + path + "/" + name
			}
			files[rel] = filename
			local := map[string]string{}
			if err := readHookDeclarations(filename, path, local); err != nil {
				return nil, err
			}
			for key, value := range local {
				if _, exists := symbols[key]; exists {
					return nil, fmt.Errorf("duplicate observed declaration %s", key)
				}
				symbols[key] = value
				locations[key] = SourceLocation{Path: rel}
			}
		}
	}
	for _, d := range decls {
		key := d.Hook.Package + "." + d.Hook.Symbol
		value, present := symbols[key]
		var loc []SourceLocation
		if present {
			loc = append(loc, locations[key])
		}
		add("declaration", declarationKey(d), present, value, loc...)
	}
	if !selection.Native {
		for _, l := range arch.Layouts {
			if _, ok := sources.packages[l.Package]; !ok {
				add("layout", []string{selection.Arch, l.Package, l.Type}, false, nil)
				continue
			}
			pkg, err := sources.Import(l.Package)
			if err != nil {
				return nil, err
			}
			present := pkg.Scope().Lookup(l.Type) != nil
			var value any
			if present {
				if obj, ok := pkg.Scope().Lookup(l.Type).(*types.TypeName); ok {
					if _, ok := obj.Type().Underlying().(*types.Struct); ok {
						value, err = describeLayout(pkg, sources.sizes, l.Type)
					} else {
						value = types.TypeString(obj.Type(), nil)
					}
				} else {
					value = "not a type"
				}
				if err != nil {
					return nil, err
				}
			}
			add("layout", []string{selection.Arch, l.Package, l.Type}, present, value)
		}
		for _, c := range arch.Constants {
			if _, ok := sources.packages[c.Package]; !ok {
				add("constant", []string{selection.Arch, c.Package, c.Symbol}, false, nil)
				continue
			}
			pkg, err := sources.Import(c.Package)
			if err != nil {
				return nil, err
			}
			obj := pkg.Scope().Lookup(c.Symbol)
			var value any
			if constant, ok := obj.(*types.Const); ok {
				value = constant.Val().ExactString()
			} else if obj != nil {
				value = "not a constant"
			}
			add("constant", []string{selection.Arch, c.Package, c.Symbol}, obj != nil, value)
		}
	}
	if req.IncludeAPIs {
		api, err := sources.observeAPIs(slices.DeleteFunc(apiPaths, func(path string) bool { _, ok := sources.packages[path]; return !ok }), InspectedContext(selection))
		if err != nil {
			return nil, err
		}
		records = append(records, api...)
	}
	if req.IncludeStdlibInventory {
		for path, pkg := range sources.packages {
			if pkg.Standard && !slices.Contains(paths, path) {
				add("package", []string{path}, true, path)
			}
		}
	}
	for path, filename := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filename)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		add("source", []string{path}, err == nil, buildspec.Hash(data), SourceLocation{Path: path})
	}
	return records, nil
}
