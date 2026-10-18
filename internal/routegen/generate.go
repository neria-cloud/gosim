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
	"github.com/neria-cloud/gosim/internal/compat/spec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

type GenerationRecord struct {
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
	if profileID != "go125" || tc.Version != "go1.25.14" {
		return fmt.Errorf("generation requires implemented go125 profile and go1.25.14")
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
	if err := p125.Verify(tc); err != nil {
		return err
	}
	for _, arch := range arches {
		sourceTC := tc
		sourceTC.HostArch = arch // VerifyHooks reads selected Linux source, not a native binary.
		if err := spec.VerifyHooks(sourceTC, p125.Descriptor().Hooks); err != nil {
			return err
		}
	}
	if err := p125.VerifyModules(tc, root); err != nil {
		return err
	}
	m, err := Load()
	if err != nil {
		return err
	}
	outputs, err := renderOutputs(tc.Version, profileID, arches, m, p125.VerificationOrigin(), p125.PolicyManifest().Modules)
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

func renderOutputs(toolchain, profile string, arches []string, m Manifest, origin spec.Origin, pins []spec.ModulePin) (map[string][]byte, error) {
	code, err := RenderKernel(m)
	if err != nil {
		return nil, err
	}
	outputs := map[string][]byte{"internal/kernel/routes_gen.go": code}
	source, err := json.Marshal(origin)
	if err != nil {
		return nil, err
	}
	for _, arch := range arches {
		// TODO(migration): Task 14.3 adds amd64 selected-source adapter generation;
		// Task 15 adds arm64. This record certifies neither wrappers nor execution.
		record := GenerationRecord{TransportDigest: buildspec.Hash(code), Schema: m.Schema, Toolchain: toolchain, Profile: profile, Architecture: arch, RouteDigest: m.Digest(), OriginDigest: buildspec.Hash(source), Modules: pins, Stage: "transport-schema"}
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
