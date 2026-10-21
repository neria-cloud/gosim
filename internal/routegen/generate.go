package routegen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/profiles"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

type GenerationRecord struct {
	AdapterDigest   string           `json:"adapter_digest,omitempty"`
	TransportDigest string           `json:"transport_digest"`
	Schema          int              `json:"schema"`
	GeneratedWith   string           `json:"generated_with"`
	Family          string           `json:"family"`
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
type GenerateOptions struct{ RefreshOrigin bool }

func GenerateProfile(tc buildspec.Toolchain, profileID string, arches []string, output string, options GenerateOptions) error {
	outputs, err := prepareProfileOutputs(tc, profileID, arches, output, options)
	if err != nil {
		return err
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

// Validation and rendering finish before publication; check uses this same map.
func prepareProfileOutputs(tc buildspec.Toolchain, profileID string, arches []string, output string, options GenerateOptions) (map[string][]byte, error) {
	if err := buildcfg.ValidateWorker(tc); err != nil {
		return nil, err
	}

	version, err := buildspec.ParseGoVersion(tc.Version)
	if err != nil {
		return nil, err
	}
	support, err := buildspec.LookupFamily(version)
	if err != nil {
		return nil, err
	}
	if support.Profile != profileID {
		return nil, fmt.Errorf("generation requires source compiler matching profile %s", profileID)
	}
	profile, err := profiles.Descriptor(profileID, tc.HostArch)
	if err != nil {
		return nil, err
	}
	policies, err := profiles.Policy(profileID)
	if err != nil {
		return nil, err
	}
	if output == "" || len(arches) == 0 {
		return nil, fmt.Errorf("explicit output and architectures are required")
	}
	seen := map[string]bool{}
	for _, arch := range arches {
		if !slices.Contains([]string{"amd64", "arm64"}, arch) || seen[arch] {
			return nil, fmt.Errorf("unsupported/duplicate simulation architecture %q", arch)
		}
		seen[arch] = true
	}
	modPath, err := buildspec.GoCommand(tc, "", "env", "GOMOD").Output()
	if err != nil {
		return nil, err
	}
	moduleFile := strings.TrimSpace(string(modPath))
	if moduleFile == "" || moduleFile == os.DevNull {
		return nil, fmt.Errorf("generation requires a module context")
	}
	root := filepath.Dir(moduleFile)
	origin, err := spec.LoadOrigin(os.DirFS(filepath.Join(root, "internal/compat/profiles")), profileID)
	if err != nil {
		return nil, err
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if err := policies.Validate(profile); err != nil {
		return nil, err
	}
	// Baseline reproduction and intentional refresh have separate provenance rules.
	if !options.RefreshOrigin {
		if err := spec.VerifyBaselineOrigin(tc, origin); err != nil {
			return nil, err
		}
		for _, arch := range arches {
			manifest, err := profile.Contracts(arch)
			if err != nil {
				return nil, err
			}
			if err := spec.VerifyFamilyContracts(context.Background(), buildspec.Resolution{Toolchain: tc}, manifest, buildspec.BuildOptions{}); err != nil {
				return nil, err
			}
		}
	}
	if options.RefreshOrigin {
		var manifests []spec.ContractManifest
		for _, arch := range []string{"amd64", "arm64"} {
			manifest, err := profile.Contracts(arch)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, manifest)
		}
		origin, err = spec.RefreshOrigin(context.Background(), buildspec.Resolution{Toolchain: tc}, origin, manifests)
		if err != nil {
			return nil, err
		}
	}
	if err := spec.VerifyModules(tc, root, policies.Modules); err != nil {
		return nil, err
	}
	m, err := Load()
	if err != nil {
		return nil, err
	}
	outputs, err := renderOutputs(tc.Version, profileID, arches, m, origin, policies.Modules)
	if err != nil {
		return nil, err
	}
	if options.RefreshOrigin {
		data, err := json.MarshalIndent(origin, "", "  ")
		if err != nil {
			return nil, err
		}
		outputs["internal/compat/profiles/"+profileID+".origin.json"] = append(data, '\n')
	}
	for _, arch := range arches {
		profile, err = profiles.Descriptor(profileID, arch)
		if err != nil {
			return nil, err
		}
		adapters, err := renderAdapters(tc, profile, arch, m, root)
		if err != nil {
			return nil, err
		}
		if len(adapters) == 0 {
			continue
		}
		recordPath := generationRecordPath(profileID, arch)
		var record GenerationRecord
		if err := json.Unmarshal(outputs[recordPath], &record); err != nil {
			return nil, err
		}
		record.Stage = "typed-adapters"
		record.AdapterDigest = adaptersDigest(adapters)
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return nil, err
		}
		outputs[recordPath] = append(data, '\n')
		for name, data := range adapters {
			outputs[name] = data
		}
	}
	return outputs, nil
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
		record := GenerationRecord{TransportDigest: buildspec.Hash(append(append([]byte(nil), code...), dispatch...)), Schema: 2, GeneratedWith: toolchain, Family: origin.Family, Profile: profile, Architecture: arch, RouteDigest: m.Digest(), OriginDigest: buildspec.Hash(source), Modules: pins, Stage: "transport-schema"}
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return nil, err
		}
		outputs[generationRecordPath(profile, arch)] = append(data, '\n')
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

func generationRecordPath(profile, arch string) string {
	return ".gosim/generation/" + profile + "/routes_" + arch + ".json"
}
