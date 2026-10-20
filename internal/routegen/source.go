package routegen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type sourcePackage struct {
	Functions map[string]*ast.FuncType
	Types     map[string]ast.Expr
	Constants map[string]ast.Expr
}

// loadSources uses the selected executable's Linux file selection, including
// separately pinned module declarations. It never uses the generator's GOROOT.
func loadSources(tc buildspec.Toolchain, arch string) (map[string]sourcePackage, error) {
	tc.HostArch = arch
	cmd := buildspec.GoCommand(tc, "", "list", "-json", "syscall", "golang.org/x/sys/unix", "internal/syscall/unix")
	for i, v := range cmd.Env {
		if strings.HasPrefix(v, "GOOS=") {
			cmd.Env[i] = "GOOS=linux"
		}
		if strings.HasPrefix(v, "CGO_ENABLED=") {
			cmd.Env[i] = "CGO_ENABLED=0"
		}
	}
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("load adapter declarations: %w", err)
	}
	result := map[string]sourcePackage{}
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var pkg struct {
			ImportPath, Dir string
			GoFiles         []string
		}
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		source := sourcePackage{map[string]*ast.FuncType{}, map[string]ast.Expr{}, map[string]ast.Expr{}}
		for _, name := range pkg.GoFiles {
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(pkg.Dir, name), nil, 0)
			if err != nil {
				return nil, err
			}
			for _, d := range file.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil {
						source.Functions[d.Name.Name] = d.Type
					}
				case *ast.GenDecl:
					for _, s := range d.Specs {
						switch s := s.(type) {
						case *ast.TypeSpec:
							source.Types[s.Name.Name] = s.Type
						case *ast.ValueSpec:
							if d.Tok == token.CONST && len(s.Names) == len(s.Values) {
								for i, n := range s.Names {
									source.Constants[n.Name] = s.Values[i]
								}
							}
						}
					}
				}
			}
		}
		result[pkg.ImportPath] = source
	}
	return result, nil
}

func sourceText(node ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), node); err != nil {
		panic(err)
	}
	return b.String()
}
