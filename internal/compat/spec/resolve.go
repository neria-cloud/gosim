package spec

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type ArchitectureRecords struct {
	Layouts   []LayoutContract
	Constants []ConstantContract
}

type ResolvedFamily struct {
	Family           string
	ProfileID        string
	Supported        bool
	Chain            []string
	DefinitionDigest string
	Declarations     []Declaration
	Packages         []PackagePolicy
	Bridges          []Bridge
	NativeImports    []NativeImport
	SharedGlobals    []SharedGlobal
	Probes           []InitProbe
	Modules          []ModulePin
	Capabilities     []Capability
	RuntimePackages  []PackageRef
	Architectures    map[string]ArchitectureRecords
}

// ResolveDefinitions is metadata-only: retained ancestors and future fixtures need
// no compiler admission. Each definition is decoded again to validate typed callers.
func ResolveDefinitions(defs map[string]FamilyDefinition, family string) (ResolvedFamily, error) {
	var out ResolvedFamily
	for name, definition := range defs {
		if name != definition.Family {
			return out, fmt.Errorf("definition map key %q differs from family %q", name, definition.Family)
		}
	}
	chain := []FamilyDefinition{}
	seen := map[string]bool{}
	for current := family; current != ""; {
		definition, ok := defs[current]
		if !ok || seen[current] {
			return out, fmt.Errorf("missing/cyclic family %q", current)
		}
		seen[current] = true
		data, err := json.Marshal(definition)
		if err != nil {
			return out, err
		}
		definition, err = DecodeDefinition(data)
		if err != nil {
			return out, fmt.Errorf("%s: %w", current, err)
		}
		minor, _ := familyMinor(current)
		expected := ""
		if minor > 25 {
			expected = fmt.Sprintf("1.%d", minor-1)
		}
		if definition.Parent != expected {
			return out, fmt.Errorf("family %s requires parent %q", current, expected)
		}
		chain = append(chain, definition)
		current = definition.Parent
	}
	if len(chain) == 0 {
		return out, fmt.Errorf("empty family chain")
	}
	slices.Reverse(chain)
	out.Architectures = map[string]ArchitectureRecords{}
	for _, layer := range chain {
		if err := applyLayer(&out, layer.Delta); err != nil {
			return ResolvedFamily{}, fmt.Errorf("%s: %w", layer.Family, err)
		}
		if err := validateDeclarationOverlap(out.Declarations); err != nil {
			return ResolvedFamily{}, err
		}
		out.Chain = append(out.Chain, layer.Family)
	}
	out.Family, out.Supported = family, defs[family].Supported
	out.ProfileID = "go" + strings.ReplaceAll(family, ".", "")
	canonical, err := json.Marshal(chain)
	if err != nil {
		return out, err
	}
	out.DefinitionDigest = buildspec.Hash(canonical)
	return out.Clone(), nil
}

// copyRecords supports only our JSON-representable metadata types. A marshal
// failure is a programmer error, not invalid definition input.
func copyRecords[T any](value T) T {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}

func (r ResolvedFamily) Clone() ResolvedFamily { return copyRecords(r) }
func (r ResolvedFamily) Canonical() ([]byte, error) {
	result := r.Clone()
	if err := applyLayer(&result, ProfileDelta{}); err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func tupleKey(parts []string) string {
	data, _ := json.Marshal(parts)
	return string(data)
}

func applyRecords[T any](base []T, delta RecordDelta[T], key func(T) []string) ([]T, error) {
	values := map[string]T{}
	for _, record := range base {
		k := tupleKey(key(record))
		if _, ok := values[k]; ok {
			return nil, fmt.Errorf("duplicate base key %s", k)
		}
		values[k] = record
	}
	seen := map[string]bool{}
	claim := func(k string) error {
		if seen[k] {
			return fmt.Errorf("key %s occurs in multiple delta operations", k)
		}
		seen[k] = true
		return nil
	}
	for _, removed := range delta.Remove {
		k := tupleKey(removed)
		if err := claim(k); err != nil {
			return nil, err
		}
		if _, ok := values[k]; !ok {
			return nil, fmt.Errorf("cannot remove missing key %s", k)
		}
		delete(values, k)
	}
	for _, replacement := range delta.Replace {
		k := tupleKey(key(replacement))
		if err := claim(k); err != nil {
			return nil, err
		}
		old, ok := values[k]
		if !ok {
			return nil, fmt.Errorf("cannot replace missing key %s", k)
		}
		if reflect.DeepEqual(old, replacement) {
			return nil, fmt.Errorf("redundant replacement %s", k)
		}
		values[k] = replacement
	}
	for _, addition := range delta.Add {
		k := tupleKey(key(addition))
		if err := claim(k); err != nil {
			return nil, err
		}
		if _, ok := values[k]; ok {
			return nil, fmt.Errorf("cannot add existing key %s", k)
		}
		values[k] = addition
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var result []T
	for _, k := range keys {
		result = append(result, values[k])
	}
	return copyRecords(result), nil
}

func mergeRecords[T any](base *[]T, delta RecordDelta[T], key func(T) []string) error {
	result, err := applyRecords(*base, delta, key)
	if err == nil {
		*base = result
	}
	return err
}

func declarationKey(d Declaration) []string {
	scope := "*"
	if len(d.Arches) == 1 {
		scope = d.Arches[0]
	}
	return []string{d.Hook.Context, d.Hook.Package, d.Hook.Symbol, scope}
}

func applyLayer(r *ResolvedFamily, d ProfileDelta) error {
	errors := []error{
		mergeRecords(&r.Declarations, d.Declarations, declarationKey),
		mergeRecords(&r.Packages, d.Packages, func(v PackagePolicy) []string { return []string{v.Path} }),
		mergeRecords(&r.Bridges, d.Bridges, func(v Bridge) []string { return []string{v.Package} }),
		mergeRecords(&r.NativeImports, d.NativeImports, func(v NativeImport) []string { return []string{v.From, v.Package} }),
		mergeRecords(&r.SharedGlobals, d.SharedGlobals, func(v SharedGlobal) []string { return []string{v.Package, v.Symbol} }),
		mergeRecords(&r.Probes, d.Probes, func(v InitProbe) []string { return []string{v.Package, v.Symbol, v.Phase, v.Operation} }),
		mergeRecords(&r.Modules, d.Modules, func(v ModulePin) []string { return []string{v.Path} }),
		mergeRecords(&r.Capabilities, d.Capabilities, func(v Capability) []string { return []string{v.Package, v.Symbol} }),
		mergeRecords(&r.RuntimePackages, d.RuntimePackages, func(v PackageRef) []string { return []string{v.Path} }),
	}
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	for _, arch := range []string{"amd64", "arm64"} {
		delta := d.Architectures[arch]
		records := r.Architectures[arch]
		if err := mergeRecords(&records.Layouts, delta.Layouts, func(v LayoutContract) []string { return []string{v.Package, v.Type} }); err != nil {
			return err
		}
		if err := mergeRecords(&records.Constants, delta.Constants, func(v ConstantContract) []string { return []string{v.Package, v.Symbol} }); err != nil {
			return err
		}
		if _, exists := r.Architectures[arch]; exists || len(records.Layouts) != 0 || len(records.Constants) != 0 {
			r.Architectures[arch] = records
		}
	}
	return nil
}

func validateDeclarationOverlap(declarations []Declaration) error {
	seen := map[string]bool{}
	for _, d := range declarations {
		for _, arch := range []string{"amd64", "arm64"} {
			if len(d.Arches) == 1 && d.Arches[0] != arch {
				continue
			}
			contexts := []string{d.Hook.Context}
			// Host contracts apply in both host modes; host-race adds race-only symbols.
			if d.Hook.Context == "host" {
				contexts = append(contexts, "host-race")
			}
			for _, context := range contexts {
				k := tupleKey([]string{context, d.Hook.Package, d.Hook.Symbol, arch})
				if seen[k] {
					return fmt.Errorf("overlapping declaration %s", k)
				}
				seen[k] = true
			}
		}
	}
	return nil
}

// Descriptor derives both contract sets independently of the callable-hook ABI.
// Production admission belongs to profiles and Profile.Validate, not this view.
func (r ResolvedFamily) Descriptor(arch, revision string) (Profile, error) {
	if arch != "amd64" && arch != "arm64" {
		return Profile{}, fmt.Errorf("unsupported simulation architecture %q", arch)
	}
	if err := validateDeclarationOverlap(r.Declarations); err != nil {
		return Profile{}, err
	}
	owned := r.Clone()
	p := Profile{
		ID: owned.ProfileID, Revision: revision, Chain: owned.Chain, DefinitionDigest: owned.DefinitionDigest,
		Support:  buildspec.FamilySupport{Family: owned.Family, Profile: owned.ProfileID},
		Packages: owned.Packages, ContractsByArch: map[string]ContractManifest{},
	}
	for _, ref := range owned.RuntimePackages {
		p.RuntimePackages = append(p.RuntimePackages, ref.Path)
	}
	for _, contractArch := range []string{"amd64", "arm64"} {
		records := owned.Architectures[contractArch]
		manifest := ContractManifest{Schema: 1, Family: owned.Family, Architecture: contractArch, Layouts: records.Layouts, Constants: records.Constants}
		for _, declaration := range owned.Declarations {
			role, err := declarationRole(declaration.Hook)
			if err != nil {
				return Profile{}, err
			}
			if !declarationApplies(declaration, contractArch) {
				continue
			}
			manifest.Hooks = append(manifest.Hooks, declaration.Hook)
			if role == "hook" && contractArch == arch {
				p.Hooks = append(p.Hooks, declaration.Hook)
			}
		}
		p.ContractsByArch[contractArch] = manifest
	}
	return p, p.validateStructure()
}

func declarationApplies(d Declaration, arch string) bool {
	return len(d.Arches) == 0 || slices.Contains(d.Arches, arch)
}

func declarationRole(h Hook) (string, error) {
	prefix := module + "/internal/compat/"
	switch h.Context {
	case "":
		for _, role := range []string{"hook", "shim"} {
			if strings.HasPrefix(h.Target, prefix+role+"s/") {
				tail := strings.TrimPrefix(h.Target, prefix+role+"s/")
				pkg, symbol, ok := strings.Cut(tail, ".")
				if ok && pkg != "" && symbol != "" && canonicalDefinitionPath(h.Target) {
					return role, nil
				}
			}
		}
	case "host", "host-race":
		if strings.HasPrefix(h.Target, prefix+"host.") && len(h.Target) > len(prefix+"host.") && canonicalDefinitionPath(h.Target) {
			return "host", nil
		}
	}
	return "", fmt.Errorf("invalid declaration context/target %q/%q", h.Context, h.Target)
}

// PolicyManifest returns independent records; permissions still need validation
// at their consumption boundary, just as for the existing profile manifests.
func (r ResolvedFamily) PolicyManifest() PolicyManifest {
	return copyRecords(PolicyManifest{SharedGlobals: r.SharedGlobals, NativeImports: r.NativeImports, Bridges: r.Bridges, Probes: r.Probes, Modules: r.Modules, Capabilities: r.Capabilities})
}

// ShimManifest is the translated package-binding view for one source ABI.
// Descriptor validates categories; an unsupported architecture has no bindings.
func (r ResolvedFamily) ShimManifest(arch string) []Hook {
	if arch != "amd64" && arch != "arm64" {
		return nil
	}
	var result []Hook
	for _, d := range r.Declarations {
		role, err := declarationRole(d.Hook)
		// Temporary old-root admission while executable shims move incrementally.
		legacy := d.Hook.Context == "" && strings.HasPrefix(d.Hook.Target, module+"/internal/compat/"+r.ProfileID+"/shims/")
		if (err == nil && role == "shim" || legacy) && declarationApplies(d, arch) {
			result = append(result, d.Hook)
		}
	}
	return result
}
