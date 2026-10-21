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

type FamilySupport struct {
	Family  string `json:"family"`
	Profile string `json:"profile"`
}

var stableVersion = regexp.MustCompile(`^go([1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// TODO(not planned): prerelease, development and alternate compiler support.
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
		return family, nil
	}
	return FamilySupport{}, fmt.Errorf("unsupported Go family %s; supported families: %v", v.Family(), supportedFamilyNames())
}

func SupportedFamilies() []FamilySupport {
	return slices.Clone(families)
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

// LookupProfile finds a registered metadata family without importing adapters.
func LookupProfile(profile string) (FamilySupport, error) {
	for _, family := range SupportedFamilies() {
		if family.Profile == profile {
			return family, nil
		}
	}
	return FamilySupport{}, fmt.Errorf("unsupported Go profile %q", profile)
}
