package spec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/types"
	"reflect"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type FieldContract struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Offset int64  `json:"offset"`
}

type LayoutContract struct {
	Package string          `json:"package"`
	Type    string          `json:"type"`
	Size    int64           `json:"size"`
	Align   int64           `json:"align"`
	Fields  []FieldContract `json:"fields"`
}

type ConstantContract struct {
	Package string `json:"package"`
	Symbol  string `json:"symbol"`
	Value   string `json:"value"`
}

type ContractManifest struct {
	Schema       int                `json:"schema"`
	Family       string             `json:"family"`
	Architecture string             `json:"architecture"`
	Hooks        []Hook             `json:"hooks"`
	Layouts      []LayoutContract   `json:"layouts"`
	Constants    []ConstantContract `json:"constants"`
}

func (p Profile) Contracts(arch string) (ContractManifest, error) {
	manifest, ok := p.ContractsByArch[arch]
	if !ok {
		return ContractManifest{}, fmt.Errorf("profile %s has no contracts for architecture %q", p.ID, arch)
	}
	if err := manifest.Validate(); err != nil {
		return ContractManifest{}, err
	}
	if manifest.Architecture != arch || manifest.Family != p.Support.Family {
		return ContractManifest{}, fmt.Errorf("contract family/architecture differs from profile %s", p.ID)
	}
	manifest.Hooks = slices.Clone(manifest.Hooks)
	manifest.Layouts = slices.Clone(manifest.Layouts)
	for i := range manifest.Layouts {
		manifest.Layouts[i].Fields = slices.Clone(manifest.Layouts[i].Fields)
	}
	manifest.Constants = slices.Clone(manifest.Constants)
	return manifest, nil
}

func (p Profile) validateContractHooks(m ContractManifest) error {
	hooks := map[string]Hook{}
	for _, h := range m.Hooks {
		if h.Context == "" {
			hooks[h.Package+"."+h.Symbol] = h
		}
	}
	for _, h := range p.Hooks {
		if got := hooks[h.Package+"."+h.Symbol]; got != h {
			return fmt.Errorf("selected hook %s.%s differs from reviewed %s contracts", h.Package, h.Symbol, m.Architecture)
		}
	}
	return nil
}

func (m ContractManifest) Validate() error {
	if m.Schema != 1 || m.Family == "" || (m.Architecture != "arm64" && m.Architecture != "amd64") || len(m.Hooks) == 0 || len(m.Layouts) == 0 || len(m.Constants) == 0 {
		return fmt.Errorf("incomplete ABI contract manifest for %s/%s", m.Family, m.Architecture)
	}
	seen := map[string]bool{}
	for _, h := range m.Hooks {
		key := "hook:" + h.Context + ":" + h.Package + "." + h.Symbol
		if !validContractPackage(h.Package) || h.Symbol == "" || h.Signature == "" || h.Target == "" || !slices.Contains([]string{"rt", "kernel", "fs", "net"}, h.Owner) || seen[key] || !slices.Contains([]string{"", "host", "host-race"}, h.Context) {
			return fmt.Errorf("invalid/duplicate contract %s", key)
		}
		seen[key] = true
	}
	for _, l := range m.Layouts {
		key := "layout:" + l.Package + "." + l.Type
		if !validContractPackage(l.Package) || l.Type == "" || l.Size < 0 || l.Align < 1 || seen[key] || len(l.Fields) == 0 {
			return fmt.Errorf("invalid/duplicate contract %s", key)
		}
		seen[key] = true
		fields := map[string]bool{}
		for _, f := range l.Fields {
			if f.Name == "" || f.Type == "" || f.Offset < 0 || f.Offset > l.Size || (f.Name != "_" && fields[f.Name]) {
				return fmt.Errorf("invalid field contract %s.%s", key, f.Name)
			}
			fields[f.Name] = true
		}
	}
	for _, c := range m.Constants {
		key := "constant:" + c.Package + "." + c.Symbol
		if !validContractPackage(c.Package) || c.Symbol == "" || c.Value == "" || seen[key] {
			return fmt.Errorf("invalid/duplicate contract %s", key)
		}
		seen[key] = true
	}
	return nil
}

func validContractPackage(path string) bool {
	return path != "" && !strings.HasPrefix(path, "-") && !strings.ContainsAny(path, "\\ \t\n") && !strings.Contains(path, "..")
}

func contractPaths(m ContractManifest, host, race bool) []string {
	var paths []string
	add := func(path string) {
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	for _, h := range m.Hooks {
		if host {
			if h.Context == "host" || (race && h.Context == "host-race") {
				add(h.Package)
			}
		} else if h.Context == "" {
			add(h.Package)
		}
	}
	if !host {
		for _, l := range m.Layouts {
			add(l.Package)
		}
		for _, c := range m.Constants {
			add(c.Package)
		}
	}
	slices.Sort(paths)
	return paths
}

func describeLayout(pkg *types.Package, sizes types.Sizes, name string) (LayoutContract, error) {
	obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok {
		return LayoutContract{}, fmt.Errorf("required type %s.%s is missing", pkg.Path(), name)
	}
	strct, ok := obj.Type().Underlying().(*types.Struct)
	if !ok {
		return LayoutContract{}, fmt.Errorf("required type %s.%s is not a struct", pkg.Path(), name)
	}
	l := LayoutContract{Package: pkg.Path(), Type: name, Size: sizes.Sizeof(obj.Type()), Align: sizes.Alignof(obj.Type())}
	fields := make([]*types.Var, strct.NumFields())
	for i := range fields {
		fields[i] = strct.Field(i)
	}
	offsets := sizes.Offsetsof(fields)
	for i, f := range fields {
		l.Fields = append(l.Fields, FieldContract{Name: f.Name(), Type: types.TypeString(types.Unalias(f.Type()), func(p *types.Package) string { return p.Path() }), Offset: offsets[i]})
	}
	return l, nil
}

func (s *sourceInspection) verifyLayoutsAndConstants(m ContractManifest) error {
	for _, want := range m.Layouts {
		pkg, err := s.Import(want.Package)
		if err != nil {
			return err
		}
		got, err := describeLayout(pkg, s.sizes, want.Type)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(got, want) {
			actual, _ := json.Marshal(got)
			expected, _ := json.Marshal(want)
			return fmt.Errorf("layout ABI drift for %s.%s in %s: got %s, expected %s", want.Package, want.Type, s.version, actual, expected)
		}
	}
	for _, want := range m.Constants {
		pkg, err := s.Import(want.Package)
		if err != nil {
			return err
		}
		obj, ok := pkg.Scope().Lookup(want.Symbol).(*types.Const)
		if !ok {
			return fmt.Errorf("required constant %s.%s is missing in %s", want.Package, want.Symbol, s.version)
		}
		if got := obj.Val().ExactString(); got != want.Value {
			return fmt.Errorf("constant ABI drift for %s.%s in %s: got %s, expected %s", want.Package, want.Symbol, s.version, got, want.Value)
		}
	}
	return nil
}

// VerifyContracts inspects declarations, never baseline file hashes or behavior.
// dir is a private inspection module with the caller's selected dependencies.
func VerifyContracts(ctx context.Context, tc buildspec.Toolchain, dir string, m ContractManifest, opts buildspec.BuildOptions) error {
	if err := m.Validate(); err != nil {
		return err
	}
	version, err := buildspec.ParseGoVersion(tc.Version)
	if err != nil || version.Family() != m.Family {
		return fmt.Errorf("contract family %s does not match %s", m.Family, tc.Version)
	}
	if opts.Experiments != "" {
		return fmt.Errorf("nondefault contract experiments are unsupported")
	}
	linux, err := inspectSources(ctx, tc, dir, SourceContext{OS: "linux", Arch: m.Architecture, Race: opts.Race}, true, contractPaths(m, false, opts.Race))
	if err != nil {
		return err
	}
	var translated, hostHooks []Hook
	for _, h := range m.Hooks {
		if h.Context == "" {
			translated = append(translated, h)
		} else if h.Context == "host" || opts.Race {
			hostHooks = append(hostHooks, h)
		}
	}
	if err := linux.verifyHooks(translated); err != nil {
		return err
	}
	if err := linux.verifyLayoutsAndConstants(m); err != nil {
		return err
	}
	if len(hostHooks) > 0 {
		host, err := inspectSources(ctx, tc, dir, SourceContext{OS: tc.HostOS, Arch: tc.HostArch, Race: opts.Race, Native: true}, false, contractPaths(m, true, opts.Race))
		if err != nil {
			return err
		}
		if err := host.verifyHooks(hostHooks); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// VerifyFamilyContracts is also used by generation, without a persistent cache.
func VerifyFamilyContracts(ctx context.Context, r buildspec.Resolution, m ContractManifest, opts buildspec.BuildOptions) (result error) {
	dir, cleanup, err := hookInspectionContext(ctx, r)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, cleanup()) }()
	return VerifyContracts(ctx, r.Toolchain, dir, m, opts)
}
