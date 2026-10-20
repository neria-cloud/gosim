package translate

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
	"unicode/utf8"

	"github.com/dave/dst"
	"golang.org/x/tools/go/packages"
)

type embeddedGlobal struct {
	name, data string
	bytes      bool
	typ        types.Type
}

// Go's loader supplies the selected embed file set. Resolve each directive
// within that set; never open a file not recorded in the content dependency hash.
func collectEmbeddedGlobals(pkg *packages.Package, files []*ast.File) ([]embeddedGlobal, error) {
	var result []embeddedGlobal
	for _, file := range files {
		dir := filepath.Dir(pkg.Fset.Position(file.Pos()).Filename)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				groups := []*ast.CommentGroup{value.Doc}
				if len(gen.Specs) == 1 {
					groups = append(groups, gen.Doc)
				}
				var patterns []string
				for _, group := range groups {
					if group == nil {
						continue
					}
					for _, comment := range group.List {
						text, ok := strings.CutPrefix(comment.Text, "//go:embed")
						first, _ := utf8.DecodeRuneInString(text)
						if !ok || !unicode.IsSpace(first) {
							continue
						}
						parsed, err := embedPatterns(text)
						if err != nil {
							return nil, err
						}
						patterns = append(patterns, parsed...)
					}
				}
				if len(patterns) == 0 {
					continue
				}
				if len(value.Names) != 1 || len(value.Values) != 0 {
					return nil, fmt.Errorf("invalid embedded declaration")
				}
				name := value.Names[0].Name
				variable, ok := pkg.TypesInfo.Defs[value.Names[0]].(*types.Var)
				if !ok {
					return nil, fmt.Errorf("untyped embedded variable %s", name)
				}
				kind := variable.Type().Underlying()
				isBytes := false
				if slice, ok := kind.(*types.Slice); ok {
					isBytes = types.Identical(slice.Elem().Underlying(), types.Typ[types.Byte])
				}
				if !isBytes && !types.Identical(kind, types.Typ[types.String]) {
					return nil, fmt.Errorf("unsupported embed type %s", variable.Type())
				}
				selected := make(map[string]bool)
				for _, pattern := range patterns {
					matched := false
					for _, candidate := range pkg.EmbedFiles {
						rel, err := filepath.Rel(dir, candidate)
						if err != nil {
							return nil, err
						}
						rel = filepath.ToSlash(rel)
						match, err := embedMatches(pattern, rel)
						if err != nil {
							return nil, err
						}
						if match {
							selected[candidate] = true
							matched = true
						}
					}
					if !matched {
						return nil, fmt.Errorf("embed pattern %q has no selected files", pattern)
					}
				}
				if len(selected) != 1 {
					return nil, fmt.Errorf("embed %s requires exactly one file, got %d", name, len(selected))
				}
				for filename := range selected {
					data, err := os.ReadFile(filename)
					if err != nil {
						return nil, err
					}
					result = append(result, embeddedGlobal{
						name: name, data: string(data), bytes: isBytes, typ: variable.Type(),
					})
				}
			}
		}
	}
	return result, nil
}

func embedPatterns(text string) ([]string, error) {
	var result []string
	for remaining := strings.TrimSpace(text); remaining != ""; remaining = strings.TrimSpace(remaining) {
		if remaining[0] == '"' || remaining[0] == '`' {
			var scan scanner.Scanner
			scan.Init(strings.NewReader(remaining))
			scan.Mode = scanner.ScanStrings | scanner.ScanRawStrings
			var failed bool
			scan.Error = func(*scanner.Scanner, string) { failed = true }
			scan.Scan()
			quoted := scan.TokenText()
			value, err := strconv.Unquote(quoted)
			if err != nil || failed {
				return nil, fmt.Errorf("invalid embed pattern %q", remaining)
			}
			remaining = remaining[len(quoted):]
			if remaining != "" && !unicode.IsSpace([]rune(remaining)[0]) {
				return nil, fmt.Errorf("invalid embed pattern separator")
			}
			result = append(result, value)
		} else {
			end := strings.IndexFunc(remaining, unicode.IsSpace)
			if end < 0 {
				end = len(remaining)
			}
			result = append(result, remaining[:end])
			remaining = remaining[end:]
		}
	}
	return result, nil
}

func embedMatches(pattern, rel string) (bool, error) {
	matchedPattern, all := strings.CutPrefix(pattern, "all:")
	if !fs.ValidPath(matchedPattern) || matchedPattern == "." {
		return false, fmt.Errorf("invalid embed pattern %q", matchedPattern)
	}
	for current := rel; current != "."; current = path.Dir(current) {
		matched, err := path.Match(matchedPattern, current)
		if err != nil {
			return false, err
		}
		if matched {
			// Direct glob matches can name hidden files. Directory traversal excludes
			// hidden descendants unless all: was explicitly requested.
			if current != rel && !all {
				for _, part := range strings.Split(strings.TrimPrefix(rel, current+"/"), "/") {
					if strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
						return false, nil
					}
				}
			}
			return true, nil
		}
		if current == "/" {
			break
		}
	}
	return false, nil
}

func (e embeddedGlobal) initializer(t *packageTranslator, container string) dst.Stmt {
	var value dst.Expr = &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(e.data)}
	if e.bytes {
		// The conversion allocates a distinct mutable slice on every machine boot.
		if types.Identical(e.typ.Underlying().(*types.Slice).Elem(), types.Typ[types.Byte]) {
			value = &dst.CallExpr{Fun: &dst.ArrayType{Elt: dst.NewIdent("byte")}, Args: []dst.Expr{value}}
		} else {
			literal := &dst.CompositeLit{Type: t.makeTypeExpr(e.typ)}
			for _, b := range []byte(e.data) {
				literal.Elts = append(literal.Elts, &dst.BasicLit{Kind: token.INT, Value: strconv.Itoa(int(b))})
			}
			value = literal
		}
	}
	return &dst.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []dst.Expr{&dst.SelectorExpr{
			X:   &dst.CallExpr{Fun: dst.NewIdent(container)},
			Sel: dst.NewIdent(e.name),
		}},
		Rhs: []dst.Expr{value},
	}
}
