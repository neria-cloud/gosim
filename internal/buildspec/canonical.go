package buildspec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

func Features(names []string) ([]string, error) {
	seen := make(map[string]bool)
	for _, name := range names {
		if name != "rt" && name != "fs" && name != "net" {
			return nil, fmt.Errorf("unknown feature %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate feature %q", name)
		}
		seen[name] = true
	}
	// TODO(not planned): fs/net under native scheduling without rt.
	if !seen["rt"] {
		return nil, fmt.Errorf("an explicit feature list including rt is required")
	}
	ordered := []string{"rt"}
	for _, name := range []string{"fs", "net"} {
		if seen[name] {
			ordered = append(ordered, name)
		}
	}
	return ordered, nil
}

func ValidatePlatform(hostOS, hostArch, simOS, simArch string) error {
	// TODO(not planned): other hosts and simulated operating systems.
	if (hostOS != "linux" || (hostArch != "amd64" && hostArch != "arm64")) &&
		(hostOS != "darwin" || hostArch != "arm64") {
		return fmt.Errorf("unsupported host %s/%s", hostOS, hostArch)
	}
	if simOS != "linux" || simArch != hostArch {
		return fmt.Errorf("simulated ABI must be linux/%s, got %s/%s", hostArch, simOS, simArch)
	}
	return nil
}

func ValidDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size && hex.EncodeToString(b) == s
}

func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// NormalizeIdentity returns an owned copy. Only the fixed build-policy tags are
// allowed: release tags and arbitrary feature-changing user tags are not inputs.
func NormalizeIdentity(id Identity) (Identity, error) {
	if id.Schema != 1 {
		return Identity{}, fmt.Errorf("unsupported identity schema %d", id.Schema)
	}
	if err := ValidateVersion(id.Toolchain); err != nil {
		return Identity{}, err
	}
	if err := ValidatePlatform(id.HostOS, id.HostArch, id.SimulationOS, id.SimulationArch); err != nil {
		return Identity{}, err
	}
	var err error
	id.Features, err = Features(id.Features)
	if err != nil {
		return Identity{}, err
	}
	if id.Experiments != "" {
		return Identity{}, fmt.Errorf("nondefault GOEXPERIMENT is unsupported")
	}
	if id.Profile != "go"+id.Toolchain[2:3]+id.Toolchain[4:6] {
		return Identity{}, fmt.Errorf("profile %q does not match %s", id.Profile, id.Toolchain)
	}
	id.Tags = slices.Clone(id.Tags)
	slices.Sort(id.Tags)
	for i, tag := range id.Tags {
		if (tag != "linkname" && tag != "sim") || (i > 0 && tag == id.Tags[i-1]) {
			return Identity{}, fmt.Errorf("unapproved or duplicate build tag %q", tag)
		}
	}
	if id.Tags == nil {
		id.Tags = []string{}
	}
	id.ProviderRevisions = slices.Clone(id.ProviderRevisions)
	slices.SortFunc(id.ProviderRevisions, func(a, b Revision) int {
		return slices.Index([]string{"fs", "net"}, a.Name) - slices.Index([]string{"fs", "net"}, b.Name)
	})
	if len(id.ProviderRevisions) != len(id.Features)-1 {
		return Identity{}, fmt.Errorf("provider revisions must match enabled optional features")
	}
	for i, rev := range id.ProviderRevisions {
		if rev.Name != id.Features[i+1] || !ValidDigest(rev.Digest) {
			return Identity{}, fmt.Errorf("invalid provider revision %q", rev.Name)
		}
	}
	if id.ProviderRevisions == nil {
		id.ProviderRevisions = []Revision{}
	}
	for _, field := range []struct{ name, value string }{
		{"distribution", id.DistributionDigest},
		{"stdlib", id.StdlibDigest},
		{"adapter", id.AdapterRevision},
		{"runtime", id.RuntimeRevision},
		{"module graph", id.ModuleGraphDigest},
		{"translator", id.TranslatorDigest},
		{"debug policy", id.DebugPolicyDigest},
	} {
		if !ValidDigest(field.value) {
			return Identity{}, fmt.Errorf("invalid %s digest", field.name)
		}
	}
	if id.Certification != "candidate" && id.Certification != "certified" {
		return Identity{}, fmt.Errorf("invalid certification status %q", id.Certification)
	}
	if (id.Certification == "candidate" && id.EvidenceDigest != "") ||
		(id.Certification == "certified" && !ValidDigest(id.EvidenceDigest)) {
		return Identity{}, fmt.Errorf("evidence digest does not match certification status")
	}
	return id, nil
}

func Canonical(id Identity) ([]byte, error) {
	id, err := NormalizeIdentity(id)
	if err != nil {
		return nil, err
	}
	return json.Marshal(id)
}

func Digest(id Identity) (string, error) {
	b, err := Canonical(id)
	if err != nil {
		return "", err
	}
	return Hash(b), nil
}

// SourceDigest binds compatibility evidence to the framework, toolchain and
// tuple. Application module graphs remain part of artifact identity, but do not
// require re-certifying unchanged simulator sources for every application.
func SourceDigest(id Identity) (string, error) {
	if _, err := NormalizeIdentity(id); err != nil {
		return "", err
	}
	id.Certification, id.EvidenceDigest = "candidate", ""
	id.ModuleGraphDigest = Hash([]byte("gosim-certification-application-graph-v1"))
	return Digest(id)
}
