package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// VerifyHooks checks declarations from the exact executable's selected Linux
// sources. Signature is go/printer's function type with parameter names removed;
// a method symbol is "Receiver.Method" (including a leading * for pointer receivers).
func VerifyHooks(tc buildspec.Toolchain, hooks []Hook) error {
	if len(hooks) == 0 {
		return fmt.Errorf("empty hook manifest")
	}
	var packages []string
	for _, hook := range hooks {
		if hook.Package == "" || strings.HasPrefix(hook.Package, "-") || strings.Contains(hook.Package, "...") {
			return fmt.Errorf("invalid hook package %q", hook.Package)
		}
		if !slices.Contains(packages, hook.Package) {
			packages = append(packages, hook.Package)
		}
	}
	slices.Sort(packages)
	args := append([]string{"list", "-json", "-tags=linkname"}, packages...)
	cmd := buildspec.GoCommand(tc, "", args...)
	// Source ABI is Linux even when the resulting executable runs on Darwin.
	for i, entry := range cmd.Env {
		if strings.HasPrefix(entry, "GOOS=") {
			cmd.Env[i] = "GOOS=linux"
		} else if strings.HasPrefix(entry, "CGO_ENABLED=") {
			cmd.Env[i] = "CGO_ENABLED=0"
		}
	}
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("load hook source using %s: %w", tc.Version, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	symbols := make(map[string]string)
	for {
		var pkg struct {
			ImportPath, Dir string
			GoFiles         []string
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		for _, file := range pkg.GoFiles {
			if err := readHookDeclarations(filepath.Join(pkg.Dir, file), pkg.ImportPath, symbols); err != nil {
				return err
			}
		}
	}
	for _, hook := range hooks {
		key := hook.Package + "." + hook.Symbol
		if got, ok := symbols[key]; !ok {
			return fmt.Errorf("required hook source %s is missing in %s", key, tc.Version)
		} else if got != hook.Signature {
			return fmt.Errorf("hook ABI drift for %s: got %q, expected %q", key, got, hook.Signature)
		}
	}
	return nil
}

func readHookDeclarations(filename, pkg string, symbols map[string]string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fn.Name.Name
		if name == "init" || name == "_" {
			continue // Not addressable hook symbols; repeated declarations are legal.
		}
		if fn.Recv != nil {
			var buf bytes.Buffer
			if err := printer.Fprint(&buf, fset, fn.Recv.List[0].Type); err != nil {
				return err
			}
			name = buf.String() + "." + name
		}
		// Expand grouped names before dropping them so (a,b int) stays (int,int).
		for _, fields := range []*ast.FieldList{fn.Type.Params, fn.Type.Results} {
			if fields == nil {
				continue
			}
			var expanded []*ast.Field
			for _, field := range fields.List {
				for i := 0; i < max(1, len(field.Names)); i++ {
					expanded = append(expanded, &ast.Field{Type: field.Type})
				}
			}
			fields.List = expanded
		}
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, token.NewFileSet(), fn.Type); err != nil {
			return err
		}
		key := pkg + "." + name
		if _, exists := symbols[key]; exists {
			return fmt.Errorf("duplicate source declaration %s", key)
		}
		symbols[key] = buf.String()
	}
	return nil
}
