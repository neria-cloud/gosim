package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/mod/sumdb/dirhash"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// VerifyModules checks the selected graph and extracted source, not merely the
// version spelling in go.mod. Replacement modules cannot silently change ABI.
func VerifyModules(tc buildspec.Toolchain, dir string, pins []ModulePin) error {
	if len(pins) == 0 {
		return fmt.Errorf("empty module ABI pins")
	}
	args := []string{"list", "-m", "-json"}
	expected := make(map[string]ModulePin)
	for _, pin := range pins {
		if expected[pin.Path].Path != "" {
			return fmt.Errorf("duplicate module pin %s", pin.Path)
		}
		expected[pin.Path] = pin
		args = append(args, pin.Path)
	}
	data, err := buildspec.GoCommand(tc, dir, args...).Output()
	if err != nil {
		return fmt.Errorf("inspect pinned modules: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var module struct {
			Path, Version, Sum, GoModSum, Dir, GoMod string
			Replace                                  *json.RawMessage
		}
		if err := decoder.Decode(&module); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		pin, ok := expected[module.Path]
		if !ok || module.Replace != nil || module.Version != pin.Version || module.Sum != pin.Sum || module.GoModSum != pin.GoModSum {
			return fmt.Errorf("module ABI pin mismatch for %s", module.Path)
		}
		if err := VerifyModuleSource(pin, module.Dir, module.GoMod); err != nil {
			return err
		}
		delete(expected, module.Path)
	}
	if len(expected) != 0 {
		return fmt.Errorf("missing pinned module records")
	}
	return nil
}

func VerifyModuleSource(pin ModulePin, dir, goMod string) error {
	if dir == "" || goMod == "" {
		return fmt.Errorf("module %s source is not available", pin.Path)
	}
	sum, err := dirhash.HashDir(dir, pin.Path+"@"+pin.Version, dirhash.Hash1)
	if err != nil {
		return err
	}
	if sum != pin.Sum {
		return fmt.Errorf("modified module source %s", pin.Path)
	}
	sum, err = dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) { return os.Open(goMod) })
	if err != nil {
		return err
	}
	if sum != pin.GoModSum {
		return fmt.Errorf("modified module definition %s", pin.Path)
	}
	return nil
}
