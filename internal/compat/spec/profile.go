// Package spec contains Go compatibility metadata, never executable adapters.
package spec

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type Profile struct {
	ID, Revision     string
	Chain            []string
	DefinitionDigest string
	Support          buildspec.FamilySupport
	Hooks            []Hook
	Packages         []PackagePolicy
	RuntimePackages  []string
	ContractsByArch  map[string]ContractManifest
}

type Hook struct {
	Package   string `json:"package"`
	Symbol    string `json:"symbol"`
	Signature string `json:"signature"`
	Target    string `json:"target"`
	Owner     string `json:"owner"`
	// Empty means translated Linux; native runtime contracts use host/host-race.
	Context string `json:"context,omitempty"`
}

type PackagePolicy struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Reason string `json:"reason"`
}

const module = "github.com/neria-cloud/gosim"

func (p Profile) Validate() error {
	canonical, err := buildspec.LookupProfile(p.ID)
	if err != nil || !reflect.DeepEqual(canonical, p.Support) {
		return fmt.Errorf("invalid profile identity/support %q", p.ID)
	}
	return p.validateStructure()
}

// validateStructure checks pure metadata without consulting caller admission.
func (p Profile) validateStructure() error {
	minor, err := familyMinor(p.Support.Family)
	if err != nil || p.Support.Profile != p.ID || p.ID != "go"+strings.ReplaceAll(p.Support.Family, ".", "") || !buildspec.ValidDigest(p.Revision) {
		return fmt.Errorf("invalid profile identity %q", p.ID)
	}
	// Legacy descriptors have neither field until the metadata facade cutover.
	if len(p.Chain) != 0 || p.DefinitionDigest != "" {
		if !buildspec.ValidDigest(p.DefinitionDigest) || len(p.Chain) != minor-24 {
			return fmt.Errorf("invalid profile ancestry %q", p.ID)
		}
		for i, family := range p.Chain {
			if family != fmt.Sprintf("1.%d", 25+i) {
				return fmt.Errorf("invalid profile chain %q", p.ID)
			}
		}
	}
	if len(p.Hooks) == 0 || len(p.Packages) == 0 || len(p.RuntimePackages) == 0 {
		return fmt.Errorf("profile %s is incomplete", p.ID)
	}

	if len(p.ContractsByArch) != 2 {
		return fmt.Errorf("profile %s has incomplete architecture contracts", p.ID)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		m, err := p.Contracts(arch)
		if err != nil {
			return err
		}
		for _, hook := range m.Hooks {
			if err := p.checkPackage(hook.Target); err != nil {
				return err
			}
		}
	}

	hooks := make(map[string]bool)
	for _, hook := range p.Hooks {
		key := hook.Package + "." + hook.Symbol
		if hook.Package == "" || hook.Symbol == "" || hook.Signature == "" || hook.Target == "" || hooks[key] {
			return fmt.Errorf("invalid/duplicate hook %s", key)
		}
		if !slices.Contains([]string{"rt", "kernel", "fs", "net"}, hook.Owner) {
			return fmt.Errorf("hook %s has unknown owner %q", key, hook.Owner)
		}
		if err := p.checkPackage(hook.Target); err != nil {
			return err
		}
		hooks[key] = true
	}
	policies := make(map[string]bool)
	for _, policy := range p.Packages {
		if policy.Path == "" || policy.Reason == "" || policies[policy.Path] ||
			!slices.Contains([]string{"translate", "native-pure", "bridge", "reject"}, policy.Mode) {
			return fmt.Errorf("invalid/duplicate package policy %q", policy.Path)
		}
		policies[policy.Path] = true
	}
	packages := make(map[string]bool)
	for _, pkg := range p.RuntimePackages {
		if pkg == "" || packages[pkg] {
			return fmt.Errorf("invalid/duplicate runtime package %q", pkg)
		}
		if err := p.checkPackage(pkg); err != nil {
			return err
		}
		packages[pkg] = true
	}
	return nil
}

func (p Profile) checkPackage(pkg string) error {
	for _, provider := range []string{"fs", "net"} {
		prefix := module + "/internal/" + provider
		if pkg == prefix || strings.HasPrefix(pkg, prefix+"/") || strings.HasPrefix(pkg, prefix+".") {
			return fmt.Errorf("profile must not unconditionally import concrete provider %s", pkg)
		}
	}
	packagePath := targetPackage(pkg)
	if _, inside := compatSuffix(packagePath); inside {
		if strings.HasPrefix(packagePath, "translated/") {
			return fmt.Errorf("profile contains relocated target %s", pkg)
		}
		kind := ClassifyCompatPackage(packagePath)
		if kind != CompatHooks && kind != CompatShims && kind != CompatHost {
			return fmt.Errorf("invalid compatibility target %s", pkg)
		}
	}

	if strings.Contains(pkg, "/internal/hooks/") {
		return fmt.Errorf("profile references retired adapter: %s", pkg)
	}

	return nil
}

func (p Profile) Select(build buildspec.Identity) (buildspec.Selection, error) {
	if err := p.Validate(); err != nil {
		return buildspec.Selection{}, err
	}
	id, err := buildspec.NormalizeIdentity(build)
	if err != nil {
		return buildspec.Selection{}, err
	}
	if id.Profile != p.ID || id.AdapterRevision != p.Revision {
		return buildspec.Selection{}, fmt.Errorf("build profile/revision differs from selected adapter")
	}
	manifest, err := p.Contracts(id.SimulationArch)
	if err != nil {
		return buildspec.Selection{}, err
	}
	if err := p.validateContractHooks(manifest); err != nil {
		return buildspec.Selection{}, err
	}
	contract, err := p.ContractDigest()
	if err != nil {
		return buildspec.Selection{}, err
	}
	if id.ContractDigest != contract {
		return buildspec.Selection{}, fmt.Errorf("build contract digest differs from selected profile")
	}
	hooks := slices.Clone(p.Hooks)
	slices.SortFunc(hooks, func(a, b Hook) int {
		if order := strings.Compare(a.Package, b.Package); order != 0 {
			return order
		}
		return strings.Compare(a.Symbol, b.Symbol)
	})
	data, err := json.Marshal(hooks)
	if err != nil {
		return buildspec.Selection{}, err
	}
	selection := buildspec.Selection{Identity: id, HookManifestDigest: buildspec.Hash(data), RuntimePackages: slices.Clone(p.RuntimePackages)}
	slices.Sort(selection.RuntimePackages)
	for _, feature := range id.Features {
		if feature != "rt" {
			selection.ProviderImports = append(selection.ProviderImports, module+"/internal/"+feature)
		}
	}
	return selection, nil
}

// ContractDigest binds declared ABI expectations, independently of baseline provenance.
func (p Profile) ContractDigest() (string, error) {
	hooks := slices.Clone(p.Hooks)
	slices.SortFunc(hooks, func(a, b Hook) int { return strings.Compare(a.Package+"."+a.Symbol, b.Package+"."+b.Symbol) })
	packages := slices.Clone(p.Packages)
	slices.SortFunc(packages, func(a, b PackagePolicy) int { return strings.Compare(a.Path, b.Path) })
	data, err := json.Marshal(struct {
		Hooks     []Hook
		Packages  []PackagePolicy
		Contracts map[string]ContractManifest
	}{hooks, packages, p.ContractsByArch})
	if err != nil {
		return "", err
	}
	return buildspec.Hash(data), nil
}
