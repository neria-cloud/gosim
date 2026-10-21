package routegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// CheckProfile renders without publication and compares complete committed bytes.
// Ignored generation records are diagnostics, never prerequisites for checking.
func CheckProfile(tc buildspec.Toolchain, profileID string, arches []string, output string) error {
	outputs, err := prepareProfileOutputs(tc, profileID, arches, output, GenerateOptions{})
	if err != nil {
		return err
	}
	return compareGeneratedOutputs(outputs, output)
}

func compareGeneratedOutputs(outputs map[string][]byte, output string) error {
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		if !strings.HasPrefix(name, ".gosim/") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		actual, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(name)))
		if err != nil {
			return fmt.Errorf("check generated %s: %w", name, err)
		}
		if !bytes.Equal(actual, outputs[name]) {
			return fmt.Errorf("stale generated output %s", name)
		}
	}
	return nil
}
