package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	modpath "golang.org/x/mod/module"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// VerifyHooks checks declarations from the exact executable's selected Linux
// sources. Signature is go/printer's function type with parameter names removed;
// a method symbol is "Receiver.Method" (including a leading * for pointer receivers).
func VerifyHooks(tc buildspec.Toolchain, hooks []Hook) (result error) {
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
	dir, cleanup, err := hookInspectionModule(tc)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, cleanup()) }()
	args := append([]string{"list", "-mod=mod", "-json", "-tags=linkname"}, packages...)
	cmd := buildspec.GoCommand(tc, dir, args...)
	// Source ABI is Linux even when the resulting executable runs on Darwin.
	for i, entry := range cmd.Env {
		if strings.HasPrefix(entry, "GOOS=") {
			cmd.Env[i] = "GOOS=linux"
		} else if strings.HasPrefix(entry, "CGO_ENABLED=") {
			cmd.Env[i] = "CGO_ENABLED=0"
		}
	}
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("load hook source using %s: %w\n%s", tc.Version, err, diagnostics.Bytes())
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

// Hook packages can be transitive dependencies absent from the application's
// direct imports. Let Go resolve those requirements in a private module using
// the same versions/replacements, without rewriting the application's go.mod.
func hookInspectionModule(tc buildspec.Toolchain) (string, func() error, error) {
	data, err := buildspec.GoCommand(tc, "", "env", "GOMOD").Output()
	if err != nil {
		return "", nil, err
	}
	path := strings.TrimSpace(string(data))
	if path == "" || path == os.DevNull {
		return "", func() error { return nil }, nil
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	mod, err := modfile.Parse(path, source, nil)
	if err != nil {
		return "", nil, err
	}
	root, original := filepath.Dir(path), mod.Module.Mod.Path
	for _, replacement := range slices.Clone(mod.Replace) {
		if replacement.New.Version == "" && !filepath.IsAbs(replacement.New.Path) {
			if err := mod.AddReplace(replacement.Old.Path, replacement.Old.Version, filepath.Join(root, replacement.New.Path), ""); err != nil {
				return "", nil, err
			}
		}
	}
	_, major, ok := modpath.SplitPathVersion(original)
	if !ok {
		return "", nil, fmt.Errorf("invalid source module %s", original)
	}
	version := "v0.0.0"
	if major != "" {
		version = strings.TrimPrefix(strings.TrimPrefix(major, "/"), ".") + ".0.0"
	}
	if err := mod.AddModuleStmt("gosim-hook-source"); err != nil {
		return "", nil, err
	}
	if err := mod.AddRequire(original, version); err != nil {
		return "", nil, err
	}
	if err := mod.AddReplace(original, "", root, ""); err != nil {
		return "", nil, err
	}
	source, err = mod.Format()
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "gosim-hook-source-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), source, 0o600); err != nil {
		return "", nil, errors.Join(err, cleanup())
	}
	if sum, err := os.ReadFile(filepath.Join(root, "go.sum")); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0o600); err != nil {
			return "", nil, errors.Join(err, cleanup())
		}
	} else if !os.IsNotExist(err) {
		return "", nil, errors.Join(err, cleanup())
	}
	return dir, cleanup, nil
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
