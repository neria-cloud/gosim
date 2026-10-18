// Package spec contains Go compatibility metadata, never executable adapters.
package spec

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type Profile struct {
	ID, Revision    string
	Tuples          []buildspec.Tuple
	Hooks           []Hook
	Packages        []PackagePolicy
	RuntimePackages []string
}

type Hook struct {
	Package   string `json:"package"`
	Symbol    string `json:"symbol"`
	Signature string `json:"signature"`
	Target    string `json:"target"`
	Owner     string `json:"owner"`
}

type PackagePolicy struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Reason string `json:"reason"`
}

const module = "github.com/neria-cloud/gosim"

func (p Profile) Validate() error {
	if (p.ID != "go125" && p.ID != "go126") || !buildspec.ValidDigest(p.Revision) {
		return fmt.Errorf("invalid profile identity/revision %q", p.ID)
	}
	if len(p.Tuples) == 0 || len(p.Hooks) == 0 || len(p.Packages) == 0 || len(p.RuntimePackages) == 0 {
		return fmt.Errorf("profile %s is incomplete", p.ID)
	}
	tuples := make(map[string]bool)
	for _, tuple := range p.Tuples {
		if err := validateTupleRecord(tuple); err != nil {
			return err
		}
		if !strings.HasPrefix(tuple.Toolchain, "go1."+p.ID[3:]+".") {
			return fmt.Errorf("tuple %s belongs to another profile", tuple.Toolchain)
		}
		key := tupleKey(tuple)
		if tuples[key] {
			return fmt.Errorf("duplicate compatibility tuple %s", key)
		}
		tuples[key] = true
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
	other := "go125"
	if p.ID == other {
		other = "go126"
	}
	if strings.Contains(pkg, "/internal/compat/"+other+"/") || strings.Contains(pkg, "/internal/hooks/") {
		return fmt.Errorf("profile %s references another/retired adapter: %s", p.ID, pkg)
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
	if err := ValidateTuple(id, p.Tuples); err != nil {
		return buildspec.Selection{}, err
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
