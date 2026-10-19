package buildspec

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

type GoVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

type PatchExclusion struct {
	First  int    `json:"first"`
	Last   int    `json:"last"`
	Reason string `json:"reason"`
}

type FamilySupport struct {
	Family       string           `json:"family"`
	Profile      string           `json:"profile"`
	MinimumPatch int              `json:"minimum_patch"`
	Exclusions   []PatchExclusion `json:"exclusions,omitempty"`
}

var stableVersion = regexp.MustCompile(`^go([1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// families is the support policy, not a list of executed/certified patches.
// TODO(not planned): prerelease, development and alternate compiler support.
var families = []FamilySupport{
	{Family: "1.25", Profile: "go125"},
	{Family: "1.26", Profile: "go126"},
}

func ParseGoVersion(raw string) (GoVersion, error) {
	parts := stableVersion.FindStringSubmatch(raw)
	if parts == nil {
		return GoVersion{}, fmt.Errorf("toolchain %q must be a stable go1.X.Y release", raw)
	}
	var v GoVersion
	for i, field := range []*int{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return GoVersion{}, fmt.Errorf("invalid Go version %q: %w", raw, err)
		}
		*field = n
	}
	return v, nil
}

func (v GoVersion) Family() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }

func LookupFamily(v GoVersion) (FamilySupport, error) {
	for _, family := range families {
		if family.Family != v.Family() {
			continue
		}
		if v.Patch < family.MinimumPatch {
			return FamilySupport{}, fmt.Errorf("Go %s requires patch %d or later", family.Family, family.MinimumPatch)
		}
		for _, excluded := range family.Exclusions {
			if v.Patch >= excluded.First && v.Patch <= excluded.Last {
				return FamilySupport{}, fmt.Errorf("unsupported Go %s.%d: %s", family.Family, v.Patch, excluded.Reason)
			}
		}
		family.Exclusions = slices.Clone(family.Exclusions)
		return family, nil
	}
	return FamilySupport{}, fmt.Errorf("unsupported Go family %s; supported families: %v", v.Family(), supportedFamilyNames())
}

func SupportedFamilies() []FamilySupport {
	result := slices.Clone(families)
	for i := range result {
		result[i].Exclusions = slices.Clone(result[i].Exclusions)
	}
	return result
}

func supportedFamilyNames() []string {
	var names []string
	for _, family := range families {
		names = append(names, family.Family)
	}
	return names
}

func ValidateVersion(raw string) error {
	v, err := ParseGoVersion(raw)
	if err != nil {
		return err
	}
	_, err = LookupFamily(v)
	return err
}
