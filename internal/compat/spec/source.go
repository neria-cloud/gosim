package spec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// SourceContext distinguishes translated Linux declarations from the native
// runtime used by the worker. Listing sources does not compile or execute them.
type SourceContext struct {
	OS, Arch string
	Race     bool
	Native   bool
}

type sourceModule struct {
	Path, Version, Dir string
	Replace            *sourceModule
}

type sourcePackage struct {
	ImportPath, Dir   string
	GoFiles, CgoFiles []string
	ImportMap         map[string]string
	Module            *sourceModule
	Error             *struct{ Err string }
}

type sourceInspection struct {
	ctx      context.Context
	packages map[string]sourcePackage
	fset     *token.FileSet
	syntax   map[string][]*ast.File
	types    map[string]*types.Package
	loading  map[string]bool
	sizes    types.Sizes
	version  string
}

func inspectSources(
	ctx context.Context,
	tc buildspec.Toolchain,
	dir string,
	selection SourceContext,
	deps bool,
	paths []string,
) (*sourceInspection, error) {
	args := []string{"list", "-mod=mod", "-json", "-tags=linkname"}
	if deps {
		args = append(args, "-deps")
	}
	args = append(args, paths...)
	cmd := buildspec.OutputCommand(ctx, tc, dir, args...)
	// These owned source tags are derived from context, never arbitrary user tags.
	// Native runtime selection follows the actual race mode without building it.
	tags := "linkname"
	if selection.Native {
		if selection.Race {
			tags += ",race"
		}
	} else {
		tags = buildspec.TranslatedSourceTags(selection.Race)
	}
	cmd.Args[4] = "-tags=" + tags
	cgo := "0"
	if selection.Native && selection.Race {
		cgo = "1"
	}
	overrides := map[string]string{"GOOS": selection.OS, "GOARCH": selection.Arch, "CGO_ENABLED": cgo}
	for i, entry := range cmd.Env {
		name, _, _ := strings.Cut(entry, "=")
		if value, ok := overrides[name]; ok {
			cmd.Env[i] = name + "=" + value
			delete(overrides, name)
		}
	}
	for _, name := range []string{"GOOS", "GOARCH", "CGO_ENABLED"} {
		if value, ok := overrides[name]; ok {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
	}
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(
			"inspect %s/%s declarations with %s: %w\n%s",
			selection.OS,
			selection.Arch,
			tc.Version,
			err,
			diagnostics.Bytes(),
		)
	}
	result := &sourceInspection{
		ctx:      ctx,
		packages: map[string]sourcePackage{},
		fset:     token.NewFileSet(),
		syntax:   map[string][]*ast.File{},
		types:    map[string]*types.Package{},
		loading:  map[string]bool{},
		sizes:    types.SizesFor("gc", selection.Arch),
		version:  tc.Version,
	}
	if result.sizes == nil {
		return nil, fmt.Errorf("unsupported source architecture %s", selection.Arch)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var pkg sourcePackage
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if pkg.Error != nil {
			return nil, fmt.Errorf("inspect %s: %s", pkg.ImportPath, pkg.Error.Err)
		}
		result.packages[pkg.ImportPath] = pkg
	}
	for _, path := range paths {
		if _, ok := result.packages[path]; !ok {
			return nil, fmt.Errorf("missing source package %s", path)
		}
	}
	return result, nil
}

func (s *sourceInspection) files(path string) ([]*ast.File, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if files, ok := s.syntax[path]; ok {
		return files, nil
	}
	pkg, ok := s.packages[path]
	if !ok {
		return nil, fmt.Errorf("missing selected source for %s", path)
	}
	var files []*ast.File
	for _, name := range pkg.GoFiles {
		f, err := parser.ParseFile(s.fset, filepath.Join(pkg.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	s.syntax[path] = files
	return files, nil
}

func (s *sourceInspection) Import(path string) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	if pkg := s.types[path]; pkg != nil {
		return pkg, nil
	}
	if s.loading[path] {
		return nil, fmt.Errorf("source import cycle at %s", path)
	}
	s.loading[path] = true
	defer delete(s.loading, path)
	files, err := s.files(path)
	if err != nil {
		return nil, err
	}
	cfg := types.Config{
		Importer:                 s,
		Sizes:                    s.sizes,
		IgnoreFuncBodies:         true,
		DisableUnusedImportCheck: true,
		GoVersion:                s.version,
	}
	pkg, err := cfg.Check(path, s.fset, files, nil)
	if err != nil {
		return nil, fmt.Errorf("inspect types in %s: %w", path, err)
	}
	s.types[path] = pkg
	return pkg, nil
}

func (s *sourceInspection) ImportFrom(path, dir string, _ types.ImportMode) (*types.Package, error) {
	for _, pkg := range s.packages {
		if pkg.Dir == dir && pkg.ImportMap[path] != "" {
			path = pkg.ImportMap[path]
			break
		}
	}
	return s.Import(path)
}

func (s *sourceInspection) verifyHooks(hooks []Hook) error {
	symbols := map[string]string{}
	var paths []string
	for _, hook := range hooks {
		if !slices.Contains(paths, hook.Package) {
			paths = append(paths, hook.Package)
		}
	}
	for _, path := range paths {
		// Read a separate AST: the signature normalizer removes parameter names.
		pkg := s.packages[path]
		for _, name := range pkg.GoFiles {
			if err := readHookDeclarations(filepath.Join(pkg.Dir, name), path, symbols); err != nil {
				return err
			}
		}
	}
	for _, hook := range hooks {
		key := hook.Package + "." + hook.Symbol
		if got, ok := symbols[key]; !ok {
			return fmt.Errorf("required hook %s is missing in %s", key, s.version)
		} else if got != hook.Signature {
			return fmt.Errorf("hook ABI drift for %s in %s: got %q, expected %q", key, s.version, got, hook.Signature)
		}
	}
	return nil
}
