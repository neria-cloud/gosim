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

func Load(path string, overrides Overrides) (buildspec.Config, error) {
	cfg := buildspec.Config{Schema: 1}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read config: %w", err)
		}
		// Require an object; null must not silently become a default config.
		if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
			return cfg, fmt.Errorf("config must be a JSON object")
		}
		cfg = buildspec.Config{}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			return cfg, fmt.Errorf("decode config: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return cfg, fmt.Errorf("config must contain exactly one JSON object")
		}
	}
	if overrides.Toolchain != nil {
		cfg.Toolchain = *overrides.Toolchain
	}
	if overrides.Features != nil {
		cfg.Features = *overrides.Features
	}
	if overrides.SimulationOS != nil {
		cfg.SimulationOS = *overrides.SimulationOS
	}
	return Normalize(cfg)
}

func Normalize(cfg buildspec.Config) (buildspec.Config, error) {
	if cfg.Schema != 1 {
		return cfg, fmt.Errorf("unsupported config schema %d", cfg.Schema)
	}
	if err := buildspec.ValidateVersion(cfg.Toolchain); err != nil {
		return cfg, err
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
