package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestReturnedErrorsAreNotLogged - the services, the Views, the themes, the
// repositories and the modules' code outside the handlers don't log an error they return: they wrap it with
// what they were doing, and the handler that deals with it logs it once
// (BaseController.LogFailure, RenderLoadFailure). An error a function
// handles itself (a fallback) is logged with LogFallback (fallback.Log) or as a warning.
func TestReturnedErrorsAreNotLogged(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	var files []string
	for _, pattern := range []string{"internal/domain/service/*.go", "internal/domain/views/*.go", "internal/domain/themes/*.go", "internal/domain/assets/*.go", "internal/infrastructure/ent/database/*.go", "internal/modules/*/*.go"} {
		m, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || isHTTPHandler(fd) {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				b, ok := n.(*ast.BlockStmt)
				if !ok {
					return true
				}
				for _, st := range b.List {
					errName := loggedError(st)
					if errName == "" {
						continue
					}
					for _, st2 := range b.List {
						if r, ok := st2.(*ast.ReturnStmt); ok && returnsIdent(r, errName) {
							t.Errorf("%s: %s logs the error it returns (%s) - wrap it instead",
								fset.Position(st.Pos()), fd.Name.Name, errName)
						}
					}
				}
				return true
			})
		}
	}
}

// isHTTPHandler - the function takes an http.ResponseWriter or a request.
func isHTTPHandler(fd *ast.FuncDecl) bool {
	for _, p := range fd.Type.Params.List {
		s := exprString(p.Type)
		if s == "http.ResponseWriter" || s == "*http.Request" {
			return true
		}
	}
	return false
}

// loggedError - the variable a logger.Error/Warn statement logs as
// "error" ("" - st isn't one).
func loggedError(st ast.Stmt) string {
	es, ok := st.(*ast.ExprStmt)
	if !ok {
		return ""
	}
	c, ok := es.X.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Error" && sel.Sel.Name != "Warn") {
		return ""
	}
	recv := exprString(sel.X)
	if !strings.HasSuffix(recv, "logger") && !strings.HasSuffix(recv, "Logger()") {
		return ""
	}
	for i := 1; i+1 < len(c.Args); i += 2 {
		if k, ok := c.Args[i].(*ast.BasicLit); ok && k.Value == `"error"` {
			if id, ok := c.Args[i+1].(*ast.Ident); ok {
				return id.Name
			}
		}
	}
	return ""
}

func returnsIdent(r *ast.ReturnStmt, name string) bool {
	for _, res := range r.Results {
		if id, ok := res.(*ast.Ident); ok && id.Name == name {
			return true
		}
	}
	return false
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.CallExpr:
		return exprString(v.Fun) + "()"
	}
	return ""
}
