package buildspec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Module records the graph Go selected, including effective local replacements
// and every workspace main module. Locations are not executable identity.
type Module struct {
	Path      string
	Version   string
	Sum       string
	GoModSum  string
	GoVersion string
	Main      bool
	Dir       string
	GoMod     string
	Replace   *Module
}

func (m Module) Effective() Module {
	if m.Replace != nil {
		return *m.Replace
	}
	return m
}

func InWorkspace(r Resolution) bool {
	return r.Context.GoWork != "" && r.Context.GoWork != "off"
}

func ApplicationFlags(r Resolution) []string {
	if InWorkspace(r) {
		return nil
	}
	return []string{"-mod=readonly"}
}

func ResolveModuleGraph(ctx context.Context, r Resolution) ([]Module, error) {
	args := append([]string{"list"}, ApplicationFlags(r)...)
	args = append(args, "-m", "-json", "all")
	cmd := ApplicationCommand(ctx, r, args...)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("resolve application module graph: %w\n%s", err, diagnostics.Bytes())
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var modules []Module
	seen := map[string]bool{}
	for {
		var m Module
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if m.Path == "" || seen[m.Path] {
			return nil, fmt.Errorf("invalid/duplicate selected module %q", m.Path)
		}
		seen[m.Path] = true
		modules = append(modules, m)
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("no application module graph; run inside a Go module or workspace")
	}
	slices.SortFunc(modules, func(a, b Module) int { return strings.Compare(a.Path, b.Path) })
	return modules, nil
}
