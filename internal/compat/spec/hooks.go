package spec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// VerifyHooks checks declarations from the exact executable's selected Linux
// sources. Signature is go/printer's function type with parameter names removed;
// a method symbol is "Receiver.Method" (including a leading * for pointer receivers).
func VerifyHooks(tc buildspec.Toolchain, hooks []Hook) (result error) {
	if len(hooks) == 0 {
		return errors.New("empty hook manifest")
	}
	var paths []string
	for _, h := range hooks {
		if !validContractPackage(h.Package) {
			return fmt.Errorf("invalid hook package %q", h.Package)
		}
		if !slices.Contains(paths, h.Package) {
			paths = append(paths, h.Package)
		}
	}
	slices.Sort(paths)
	dir, cleanup, err := hookInspectionModule(tc)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, cleanup()) }()
	sources, err := inspectSources(
		context.Background(),
		tc,
		dir,
		SourceContext{OS: "linux", Arch: tc.HostArch},
		false,
		paths,
	)
	if err != nil {
		return err
	}
	return sources.verifyHooks(hooks)
}

// Hook packages can be transitive dependencies absent from the application's
// direct imports. Let Go resolve those requirements in a private module using
// the same versions/replacements, without rewriting the application's go.mod.
func hookInspectionModule(tc buildspec.Toolchain) (string, func() error, error) {
	return hookInspectionContext(context.Background(), buildspec.Resolution{Toolchain: tc})
}

func hookInspectionContext(ctx context.Context, r buildspec.Resolution) (string, func() error, error) {
	if !buildspec.InWorkspace(r) {
		data, err := buildspec.ApplicationCommand(ctx, r, "env", "GOMOD").Output()
		if err != nil {
			return "", nil, err
		}
		path := strings.TrimSpace(string(data))
		if path == "" || path == os.DevNull {
			return "", func() error { return nil }, nil
		}
	}
	modules, err := buildspec.ResolveModuleGraph(ctx, r)
	if err != nil {
		return "", nil, err
	}
	files, err := buildspec.ModuleFiles(r, modules, "gosim-hook-source")
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "gosim-hook-source-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return "", nil, errors.Join(err, cleanup())
		}
	}
	output := r
	output.Context = buildspec.ModuleContext{Directory: dir, GoWork: "off", GoMod: filepath.Join(dir, "go.mod")}
	selected, err := buildspec.ResolveModuleGraph(ctx, output)
	if err == nil {
		err = buildspec.MatchModuleGraph(modules, selected, "gosim-hook-source")
	}
	if err != nil {
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
