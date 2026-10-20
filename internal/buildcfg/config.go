// Package buildcfg resolves immutable host-side build settings.
package buildcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type Overrides struct {
	Toolchain    *string
	Features     *[]string
	SimulationOS *string
}

type ConfigRequest struct {
	Config            buildspec.Config
	ExplicitToolchain string
	Legacy            bool
}

// Load distinguishes Go discovery (the default) from an explicit legacy/CLI override.
// Legacy is exposed to the command layer so its migration diagnostic stays on stderr.
func Load(path string, overrides Overrides) (ConfigRequest, error) {
	request := ConfigRequest{
		Config: buildspec.Config{Schema: 2, Features: []string{"rt", "fs", "net"}, SimulationOS: "linux"},
	}
	if path != "" {
		if err := loadFileConfig(path, &request); err != nil {
			return request, err
		}
	}
	if overrides.Toolchain != nil {
		request.ExplicitToolchain = *overrides.Toolchain
	}
	if request.ExplicitToolchain != "" {
		if err := buildspec.ValidateVersion(request.ExplicitToolchain); err != nil {
			return request, err
		}
	}
	if overrides.Features != nil {
		request.Config.Features = *overrides.Features
	}
	if overrides.SimulationOS != nil {
		request.Config.SimulationOS = *overrides.SimulationOS
	}
	cfg, err := Normalize(request.Config)
	request.Config = cfg
	return request, err
}

func loadFileConfig(path string, request *ConfigRequest) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return fmt.Errorf("config must be a JSON object")
	}
	var wire struct {
		Schema       int      `json:"schema"`
		Toolchain    *string  `json:"toolchain"`
		Features     []string `json:"features"`
		SimulationOS string   `json:"simulation_os"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("config must contain exactly one JSON object")
	}
	if wire.Schema != 1 && wire.Schema != 2 {
		return fmt.Errorf("unsupported config schema %d", wire.Schema)
	}
	if wire.Schema == 2 && wire.Toolchain != nil {
		return fmt.Errorf("schema 2 uses Go tooling for compiler selection; remove toolchain or use explicit -go")
	}
	if wire.Schema == 1 {
		if wire.Toolchain == nil || *wire.Toolchain == "" {
			return fmt.Errorf(
				"legacy schema 1 requires an exact toolchain; migrate to schema 2 for automatic Go selection",
			)
		}
		request.ExplicitToolchain, request.Legacy = *wire.Toolchain, true
	}
	request.Config = buildspec.Config{Schema: 2, Features: wire.Features, SimulationOS: wire.SimulationOS}
	return nil
}

func Normalize(cfg buildspec.Config) (buildspec.Config, error) {
	if cfg.Schema != 2 {
		return cfg, fmt.Errorf("unsupported config schema %d", cfg.Schema)
	}
	features, err := buildspec.Features(cfg.Features)
	if err != nil {
		return cfg, err
	}
	cfg.Features = features
	if cfg.SimulationOS == "" {
		cfg.SimulationOS = "linux"
	}
	if cfg.SimulationOS != "linux" {
		return cfg, fmt.Errorf("unsupported simulation_os %q", cfg.SimulationOS)
	}
	return cfg, nil
}
