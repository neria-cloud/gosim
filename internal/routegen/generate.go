package routegen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/neria-cloud/gosim/internal/buildspec"
	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

type GenerationRecord struct {
	AdapterDigest   string           `json:"adapter_digest,omitempty"`
	TransportDigest string           `json:"transport_digest"`
	Schema          int              `json:"schema"`
	Toolchain       string           `json:"toolchain"`
	Profile         string           `json:"profile"`
	Architecture    string           `json:"architecture"`
	RouteDigest     string           `json:"route_digest"`
	OriginDigest    string           `json:"origin_digest"`
	Modules         []spec.ModulePin `json:"modules"`
	Stage           string           `json:"stage"`
}

// GenerateProfile requires the selected patch, family, architectures and output
// root. The current module graph is verified independently of the output path.
// It never changes tuple certification, source ABI expectations, or another
// family's files. Shared route schemas are architecture-neutral.
func GenerateProfile(tc buildspec.Toolchain, profileID string, arches []string, output string) error {
	var profile spec.Profile
	var origin spec.Origin
	var policies spec.PolicyManifest
	var verify func(buildspec.Toolchain) error
	var descriptor func(string) (spec.Profile, error)
	switch {
	case profileID == "go125" && tc.Version == "go1.25.14":
		descriptor = p125.DescriptorForArch
		profile, origin, policies, verify = p125.Descriptor(), p125.VerificationOrigin(), p125.PolicyManifest(), p125.Verify
	case profileID == "go126" && tc.Version == "go1.26.9":
		descriptor = p126.DescriptorForArch
		profile, origin, policies, verify = p126.Descriptor(), p126.VerificationOrigin(), p126.PolicyManifest(), p126.Verify
	default:
		return fmt.Errorf("generation requires a matching implemented profile and exact pinned toolchain")
	}
	if output == "" || len(arches) == 0 {
		return fmt.Errorf("explicit output and architectures are required")
	}
	seen := map[string]bool{}
	for _, arch := range arches {
		if !slices.Contains([]string{"amd64", "arm64"}, arch) || seen[arch] {
			return fmt.Errorf("unsupported/duplicate simulation architecture %q", arch)
		}
		seen[arch] = true
	}
	root, err := gosimtool.FindGoModDir()
	if err != nil {
		return err
	}
	if err := verify(tc); err != nil {
		return err
	}
	for _, arch := range arches {
		sourceTC := tc
		sourceTC.HostArch = arch // VerifyHooks reads selected Linux source, not a native binary.
		archProfile, err := descriptor(arch)
		if err != nil {
			return err
		}
		if err := spec.VerifyHooks(sourceTC, archProfile.Hooks); err != nil {
			return err
		}
	}
	if err := spec.VerifyModules(tc, root, policies.Modules); err != nil {
		return err
	}
	m, err := Load()
	if err != nil {
		return err
	}
	outputs, err := renderOutputs(tc.Version, profileID, arches, m, origin, policies.Modules)
	if err != nil {
		return err
	}
	for _, arch := range arches {
		profile, err = descriptor(arch)
		if err != nil {
			return err
		}
		adapters, err := renderAdapters(tc, profile, arch, m)
		if err != nil {
			return err
		}
		if len(adapters) == 0 {
			continue
		}
		recordPath := "internal/compat/" + profileID + "/profile/routes_" + arch + ".json"
		var record GenerationRecord
		if err := json.Unmarshal(outputs[recordPath], &record); err != nil {
			return err
		}
		record.Stage = "typed-adapters"
		record.AdapterDigest = adaptersDigest(adapters)
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		outputs[recordPath] = append(data, '\n')
		for name, data := range adapters {
			outputs[name] = data
		}
	}
	// Validate/render all inputs before the first write, then write in stable order.
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := writeFile(filepath.Join(output, filepath.FromSlash(name)), outputs[name]); err != nil {
			return err
		}
	}
	return nil
}

func renderOutputs(toolchain, profile string, arches []string, m Manifest, origin spec.Origin, pins []spec.ModulePin) (map[string][]byte, error) {
	code, err := RenderKernel(m)
	if err != nil {
		return nil, err
	}
	dispatch, err := renderDispatch(m)
	if err != nil {
		return nil, err
	}
	outputs := map[string][]byte{"internal/kernel/routes_gen.go": code, "internal/kernel/dispatch_gen.go": dispatch}
	source, err := json.Marshal(origin)
	if err != nil {
		return nil, err
	}
	for _, arch := range arches {
		// GenerateProfile augments this transport record with selected adapter
		// source digests; the record itself never certifies execution.
		record := GenerationRecord{TransportDigest: buildspec.Hash(append(append([]byte(nil), code...), dispatch...)), Schema: m.Schema, Toolchain: toolchain, Profile: profile, Architecture: arch, RouteDigest: m.Digest(), OriginDigest: buildspec.Hash(source), Modules: pins, Stage: "transport-schema"}
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return nil, err
		}
		outputs["internal/compat/"+profile+"/profile/routes_"+arch+".json"] = append(data, '\n')
	}
	return outputs, nil
}

func writeFile(path string, data []byte) error {
	previous, err := os.ReadFile(path)
	if err == nil && bytes.Equal(previous, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".gosim-gen-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func familyBounds(profile string) string {
	switch profile {
	case "go125":
		return "go1.25 && !go1.26"
	case "go126":
		return "go1.26 && !go1.27"
	default:
		panic("invalid generation family")
	}
}
