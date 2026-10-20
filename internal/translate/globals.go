package translate

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func (t *packageTranslator) rewriteJSONGlobalsHack(c *dstutil.Cursor) {
	if decl, ok := c.Node().(*dst.FuncDecl); ok {
		if t.pkgPath == "encoding/json" && (decl.Name.Name == "cachedTypeFields" || decl.Name.Name == "typeEncoder") {
			decl.Body.List = append([]dst.Stmt{
				&dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: t.newRuntimeSelector("BeginControlledNondeterminism"),
					},
				},
				&dst.DeferStmt{
					Call: &dst.CallExpr{
						Fun: t.newRuntimeSelector("EndControlledNondeterminism"),
					},
				},
			}, decl.Body.List...)
		}
	}
}

func (t *packageTranslator) rewriteInit(c *dstutil.Cursor) {
	node := c.Node()

	decl, ok := node.(*dst.FuncDecl)
	if !ok {
		return
	}

	if decl.Name.Name != "init" || decl.Recv != nil {
		return
	}

	astIdent, ok := t.astMap.Nodes[decl.Name].(*ast.Ident)
	if !ok {
		// XXX hmmm??? why does this happen??
		return
	}
	obj, ok := t.typesInfo.Defs[astIdent]
	if !ok {
		return
	}
	variable, ok := obj.(*types.Func)
	if !ok {
		return
	}
	pkg := variable.Pkg()
	if pkg == nil {
		return
	}
	if variable.Parent() != pkg.Scope() {
		return
	}

	idx := t.collect.initIdx
	t.collect.initIdx++
	var forTest string
	if t.forTest {
		forTest = "fortest"
	}
	name := fmt.Sprintf("gosiminit%s%d", forTest, idx)
	t.collect.inits = append(t.collect.inits, name)

	decl.Name = dst.NewIdent(name)
}

func (t *packageTranslator) isGlobalUse(node dst.Node) (dst.Expr, bool) {
	expr, ok := node.(dst.Expr)
	if !ok {
		return nil, false
	}
	expr = dstutil.Unparen(expr)

	ident, ok := expr.(*dst.Ident)
	if !ok {
		return nil, false
	}

	astExpr, ok := t.astMap.Nodes[ident].(ast.Expr) // XXX: use expr here because a dst ident might be an ast selector; happens in other places also?
	if !ok {
		return nil, false
	}

	astIdent, ok := astExpr.(*ast.Ident)
	if !ok {
		astSelector, ok := astExpr.(*ast.SelectorExpr)
		if !ok {
			return nil, false
		}
		astIdent = astSelector.Sel
	}

	obj, ok := t.typesInfo.Uses[astIdent]
	if !ok {
		return nil, false
	}

	variable, ok := obj.(*types.Var)
	if !ok {
		return nil, false
	}

	// XXX: jank
	pkg := variable.Pkg()
	if pkg == nil {
		return nil, false
	}

	if variable.Parent() != pkg.Scope() {
		return nil, false
	}

	path := pkg.Path()
	if t.nativeImports[path] {
		return nil, false
	}
	name := variable.Name()
	if t.profile != nil {
		if replacement, ok := t.profile.replacements[PackageSelector{Pkg: path, Selector: name}]; ok {
			path, name = replacement.Pkg, replacement.Selector
		} else if replacement := t.profile.packages[path]; replacement != "" {
			path = replacement
		}
	}
	if pkgInfo, ok := t.globalInfo.ByPackage[path]; ok {
		if container, ok := pkgInfo.Container[name]; ok {
			return &dst.SelectorExpr{
				X: &dst.CallExpr{
					Fun: &dst.Ident{Name: container, Path: ident.Path},
				},
				Sel: dst.NewIdent(ident.Name),
			}, true
		}
	}

	return nil, false
}

func (t *packageTranslator) rewriteGlobalRead(c *dstutil.Cursor) {
	if replacement, ok := t.isGlobalUse(c.Node()); ok {
		c.Replace(replacement)
	}
}

func (t *packageTranslator) rewriteGlobalDef(c *dstutil.Cursor) {
	file, ok := c.Node().(*dst.File)
	if !ok {
		return
	}

	var filtered []dst.Decl

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok {
			filtered = append(filtered, decl)
			continue
		}
		if genDecl.Tok != token.VAR {
			filtered = append(filtered, decl)
			continue
		}
		t.collectGlobalDefImpl(genDecl)
		if len(genDecl.Specs) != 0 {
			// XXX: for kept specs
			filtered = append(filtered, genDecl)
		}
	}

	file.Decls = filtered
}

func (t *packageTranslator) collectGlobalDefImpl(decl *dst.GenDecl) {
	var filtered []dst.Spec

	for _, spec := range decl.Specs {
		spec := spec.(*dst.ValueSpec)

		for _, name := range spec.Names {
			obj, ok := t.typesInfo.Defs[t.astMap.Nodes[name].(*ast.Ident)]
			if !ok {
				return
			}
			variable, ok := obj.(*types.Var)
			if !ok {
				return
			}

			if name.Name == "_" {
				continue
			}

			if _, ok := t.globalInfo.ByPackage[t.pkgPath].Container[name.Name]; !ok {
				if ok := t.globalInfo.ByPackage[t.pkgPath].ShouldShare[name.Name]; !ok {
					filtered = append(filtered, spec)
					continue
				}
			}

			if !t.globalInfo.ByPackage[t.pkgPath].ShouldShare[name.Name] {
				t.collect.globalFields = append(t.collect.globalFields, &dst.Field{
					Names: []*dst.Ident{name},
					Type:  t.makeTypeExpr(variable.Type()),
				})
			} else {
				t.collect.sharedGlobalFields = append(t.collect.sharedGlobalFields, &dst.ValueSpec{
					Names: []*dst.Ident{name},
					Type:  t.makeTypeExpr(variable.Type()),
				})
			}
		}
	}

	decl.Specs = filtered
}

func makeDetgoGlobalsFile(t *packageTranslator, pkgName string, forTest bool) (*dst.File, error) {
	var inits []dst.Stmt

	// XXX: somehow keep the initializers closer to their old location in the code?

	// TODO: Consider if we can group the globals in a combined struct with
	// anonymous members. That might make the source of the globals a nice
	// implementation detail.

	funcName := "G"
	typName := "Globals"
	varName := "globals"
	initFuncName := "initializers"
	field := "Globals"
	if forTest {
		funcName = "GForTest"
		typName = "GlobalsForTest"
		varName = "globalsForTest"
		initFuncName = "initializersForTest"
		field = "ForTest"
	}

	for _, embedded := range t.embedded {
		inits = append(inits, embedded.initializer(t, funcName))
	}

	var lastIfSmt *dst.BlockStmt

	for _, initializer := range t.typesInfo.InitOrder {
		var lhs []dst.Expr

		shouldShare := t.globalInfo.ByPackage[t.pkgPath].ShouldShare[initializer.Lhs[0].Name()]

		for _, variable := range initializer.Lhs {
			if variable.Name() == "_" {
				lhs = append(lhs, dst.NewIdent("_"))
			} else {
				if !shouldShare {
					lhs = append(lhs, &dst.SelectorExpr{
						X:   &dst.CallExpr{Fun: dst.NewIdent(funcName)},
						Sel: dst.NewIdent(variable.Name()),
					})
				} else {
					lhs = append(lhs, dst.NewIdent(variable.Name()))
				}
			}
		}

		dstNode := t.dstMap.Nodes[initializer.Rhs]
		if dstNode == nil {
			fmt.Printf(
				"globals: nil dstMap node for initializer in pkg=%s lhs=%s rhs=%v\n",
				t.pkgPath,
				initializer.Lhs[0].Name(),
				initializer.Rhs,
			)
			continue
		}
		rhs := t.apply(dstNode).(dst.Expr) // hmm

		if shouldShare {
			if lastIfSmt == nil {
				lastIfSmt = &dst.BlockStmt{}
				inits = append(inits, &dst.IfStmt{
					Cond: dst.NewIdent("initializeShared"),
					Body: lastIfSmt,
				})
			}
			lastIfSmt.List = append(lastIfSmt.List, &dst.AssignStmt{
				Lhs: lhs,
				Tok: token.ASSIGN,
				Rhs: []dst.Expr{
					rhs,
				},
			})
			if callExpr, ok := rhs.(*dst.CallExpr); ok {
				if ident, ok := callExpr.Fun.(*dst.Ident); ok && ident.Name == "NewReplacer" &&
					ident.Path == t.replacedPkgs["strings"] {
					lastIfSmt.List = append(lastIfSmt.List, &dst.ExprStmt{
						X: &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X:   dst.NewIdent(initializer.Lhs[0].Name()),
								Sel: dst.NewIdent("Replace"),
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Kind:  token.STRING,
									Value: `""`,
								},
							},
						},
						Decs: dst.ExprStmtDecorations{
							NodeDecs: dst.NodeDecs{
								End: []string{"// trigger once initialization"},
							},
						},
					})
				}
			}
		} else {
			inits = append(inits, &dst.AssignStmt{
				Lhs: lhs,
				Tok: token.ASSIGN,
				Rhs: []dst.Expr{
					rhs,
				},
			})
			lastIfSmt = nil
		}
	}

	for _, name := range t.collect.inits {
		inits = append(inits, &dst.ExprStmt{
			X: &dst.CallExpr{
				Fun: dst.NewIdent(name),
			},
		})
	}

	dstFile := &dst.File{
		Name: dst.NewIdent(pkgName),
		Decls: []dst.Decl{
			&dst.GenDecl{
				Tok: token.TYPE,
				Specs: []dst.Spec{
					&dst.TypeSpec{
						Name: dst.NewIdent(typName),
						Type: &dst.StructType{
							Fields: &dst.FieldList{
								List: t.collect.globalFields,
							},
						},
					},
				},
			},
			&dst.GenDecl{
				Tok: token.VAR,
				Specs: append([]dst.Spec{
					&dst.ValueSpec{
						Names: []*dst.Ident{dst.NewIdent(varName)},
						Type: &dst.IndexListExpr{
							X:       t.newRuntimeSelector("Global"),
							Indices: []dst.Expr{dst.NewIdent(typName)},
						},
					},
				}, t.collect.sharedGlobalFields...),
			},
			&dst.FuncDecl{
				Name: dst.NewIdent(initFuncName),
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							{
								Names: []*dst.Ident{dst.NewIdent("initializeShared")},
								Type:  dst.NewIdent("bool"),
							},
						},
					},
				},
				Body: &dst.BlockStmt{
					List: inits,
				},
			},
			&dst.FuncDecl{
				Name: dst.NewIdent(funcName),
				Type: &dst.FuncType{
					Results: &dst.FieldList{
						List: []*dst.Field{
							{
								Type: &dst.StarExpr{X: dst.NewIdent(typName)},
							},
						},
					},
				},
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						&dst.ReturnStmt{
							Results: []dst.Expr{
								&dst.CallExpr{
									Fun: &dst.SelectorExpr{
										X:   dst.NewIdent(varName),
										Sel: dst.NewIdent("Get"),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	globals := &dst.CompositeLit{
		Type: t.newRuntimeSelector("Globals"),
		Elts: []dst.Expr{
			&dst.KeyValueExpr{
				Key: dst.NewIdent("Globals"),
				Value: &dst.UnaryExpr{
					Op: token.AND,
					X:  dst.NewIdent(varName),
				},
			},
			&dst.KeyValueExpr{
				Key:   dst.NewIdent("Initializers"),
				Value: dst.NewIdent(initFuncName),
			},
		},
	}
	if t.infrastructure && !forTest {
		globals.Elts = append(
			globals.Elts,
			&dst.KeyValueExpr{Key: dst.NewIdent("Infrastructure"), Value: dst.NewIdent("true")},
		)
	}

	dstFile.Decls = append(dstFile.Decls, &dst.FuncDecl{
		Name: dst.NewIdent("init"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.AssignStmt{
					Lhs: []dst.Expr{
						&dst.SelectorExpr{
							X: &dst.CallExpr{
								Fun: t.newRuntimeSelector("RegisterPackage"),
								Args: []dst.Expr{
									&dst.BasicLit{
										Kind:  token.STRING,
										Value: strconv.Quote(t.pkgPath),
									},
								},
							},
							Sel: dst.NewIdent(field),
						},
					},
					Tok: token.ASSIGN,
					Rhs: []dst.Expr{
						&dst.UnaryExpr{
							Op: token.AND,
							X:  globals,
						},
					},
				},
			},
		},
	})

	dstFile.Decls = append(dstFile.Decls, makeBindFuncs(t.collect.bindspecs)...)

	return dstFile, nil
}

// len/cap of an array can be constant even when the array is a mutable global.
// Replacing that global with G().field introduces a call, which would destroy
// constant-expression semantics. Preserve the type checker's constant value.
func (t *packageTranslator) rewriteConstantArrayLen(c *dstutil.Cursor) bool {
	call, ok := c.Node().(*dst.CallExpr)
	if !ok {
		return false
	}
	original, ok := t.astMap.Nodes[call].(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := original.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := t.typesInfo.Uses[ident].(*types.Builtin)
	if !ok || (builtin.Name() != "len" && builtin.Name() != "cap") {
		return false
	}
	value := t.typesInfo.Types[original].Value
	if value == nil {
		return false
	}
	literal := &dst.BasicLit{Kind: token.INT, Value: value.ExactString()}
	literal.Decs.NodeDecs = call.Decs.NodeDecs
	c.Replace(literal)
	return true
}
