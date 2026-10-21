package spec

import "strings"

type CompatPackageKind uint8

const (
	CompatOutside CompatPackageKind = iota
	CompatHooks
	CompatShims
	CompatHost
	CompatMetadata
	CompatTooling
	CompatUnknown
)

// ClassifyCompatPackage recognizes categories, not permissions. Native exemptions
// and selection still require their exact metadata at the consumption boundary.
func ClassifyCompatPackage(path string) CompatPackageKind {
	suffix, inside := compatSuffix(path)
	if !inside {
		return CompatOutside
	}
	if !canonicalDefinitionPath(path) {
		return CompatUnknown
	}
	// Transitional family roots disappear with the old metadata facade.
	first, rest, _ := strings.Cut(suffix, "/")
	if first == "go125" || first == "go126" {
		suffix = rest
	}
	category, rest, _ := strings.Cut(suffix, "/")
	switch category {
	case "hooks":
		if rest != "" {
			return CompatHooks
		}
	case "shims":
		if rest != "" {
			return CompatShims
		}
	case "host":
		if rest == "" {
			return CompatHost
		}
	case "profiles", "profile", "spec", "selected":
		if rest == "" {
			return CompatMetadata
		}
	case "audit", "genprofile", "catalog":
		if rest == "" {
			return CompatTooling
		}
	}
	return CompatUnknown
}

func compatSuffix(path string) (string, bool) {
	for _, prefix := range []string{module + "/internal/compat/", "translated/" + module + "/internal_/compat/"} {
		if path == strings.TrimSuffix(prefix, "/") {
			return "", true
		}
		if suffix, ok := strings.CutPrefix(path, prefix); ok {
			return suffix, true
		}
	}
	return "", false
}

func targetPackage(target string) string {
	start := strings.LastIndex(target, "/") + 1
	if dot := strings.IndexByte(target[start:], '.'); dot >= 0 {
		return target[:start+dot]
	}
	return target
}

func (p Profile) translatedCompatPackage(path string) bool {
	if !strings.HasPrefix(path, module+"/internal/compat/") {
		return false
	}
	kind := ClassifyCompatPackage(path)
	if kind != CompatHooks && kind != CompatShims {
		return false
	}
	suffix, _ := compatSuffix(path)
	family, _, _ := strings.Cut(suffix, "/")
	return !strings.HasPrefix(family, "go") || family == p.ID
}
