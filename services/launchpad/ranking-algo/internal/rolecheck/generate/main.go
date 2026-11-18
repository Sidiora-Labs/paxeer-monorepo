// Command generate refreshes SQL call locations from the runtime Go sources.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type source struct {
	functions map[string]*ast.FuncDecl
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func literal(expr ast.Expr) string {
	v, ok := expr.(*ast.BasicLit)
	if !ok || v.Kind != token.STRING {
		panic("string literal required")
	}
	text, err := strconv.Unquote(v.Value)
	must(err)
	return text
}

func quoted(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func calls(fn *ast.FuncDecl) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if ok && len(call.Args) >= 2 {
			switch method.Sel.Name {
			case "Exec", "Query", "QueryRow", "CopyFrom":
				out = append(out, call)
			}
		}
		return true
	})
	return out
}

func (s source) implementation(name string, seen map[string]bool) *ast.FuncDecl {
	if seen[name] {
		panic("cyclic SQL wrapper: " + name)
	}
	seen[name] = true
	fn := s.functions[name]
	if fn == nil {
		panic("SQL function absent: " + name)
	}
	if len(calls(fn)) > 0 {
		return fn
	}
	var target *ast.FuncDecl
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || s.functions[method.Sel.Name] == nil {
			return true
		}
		if target != nil {
			panic("ambiguous SQL wrapper: " + name)
		}
		target = s.implementation(method.Sel.Name, seen)
		return true
	})
	if target == nil {
		panic("SQL wrapper target absent: " + name)
	}
	return target
}

func main() {
	root, err := os.Getwd()
	must(err)
	for {
		if _, err := os.Stat(filepath.Join(root, "migrations", "registry.json")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			panic("repository root absent")
		}
		root = parent
	}
	path := filepath.Join(root, "services/launchpad/ranking-algo/internal/rolecheck/inventory.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	must(err)
	constants := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		if spec, ok := node.(*ast.ValueSpec); ok && len(spec.Names) == 1 && len(spec.Values) == 1 {
			if v, ok := spec.Values[0].(*ast.BasicLit); ok && v.Kind == token.STRING {
				constants[spec.Names[0].Name] = literal(v)
			}
		}
		return true
	})
	var statements *ast.CompositeLit
	ast.Inspect(file, func(node ast.Node) bool {
		if spec, ok := node.(*ast.ValueSpec); ok && len(spec.Names) == 1 && spec.Names[0].Name == "Statements" {
			statements = spec.Values[0].(*ast.CompositeLit)
		}
		return true
	})
	if statements == nil {
		panic("statement inventory absent")
	}
	type group struct {
		path    ast.Expr
		fn      *ast.FuncDecl
		entries []*ast.CallExpr
	}
	groups := map[string]*group{}
	var order []string
	var inventoryCalls []*ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		entry, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := entry.Fun.(*ast.Ident)
		if ok && name.Name == "statement" {
			inventoryCalls = append(inventoryCalls, entry)
		}
		return true
	})
	for _, entry := range inventoryCalls {
		var sourcePath string
		switch expr := entry.Args[1].(type) {
		case *ast.BasicLit:
			sourcePath = literal(expr)
		case *ast.Ident:
			sourcePath = constants[expr.Name]
		case *ast.BinaryExpr:
			sourcePath = constants[expr.X.(*ast.Ident).Name] + literal(expr.Y)
		default:
			panic("unsupported source path")
		}
		function := literal(entry.Args[2])
		key := sourcePath + ":" + function
		g := groups[key]
		if g == nil {
			parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, sourcePath), nil, 0)
			must(err)
			s := source{functions: map[string]*ast.FuncDecl{}}
			for _, decl := range parsed.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					s.functions[fn.Name.Name] = fn
				}
			}
			g = &group{path: entry.Args[1], fn: s.implementation(function, map[string]bool{})}
			groups[key] = g
			order = append(order, key)
		}
		g.entries = append(g.entries, entry)
	}
	for _, key := range order {
		g := groups[key]
		sqlCalls := calls(g.fn)
		missing := len(sqlCalls) - len(g.entries)
		if missing < 0 {
			panic("inventory exceeds source calls: " + key)
		}
		// New parameter-free clock reads precede the parameterized universe
		// reads. Their function privileges are derived from the SQL itself.
		for i := 0; i < missing; i++ {
			call := sqlCalls[i]
			if len(call.Args) != 2 {
				panic("new statement requires reviewed relation privileges: " + key)
			}
			sql := literal(call.Args[1])
			if !strings.HasPrefix(sql, "SELECT ") || strings.Contains(strings.ToUpper(sql), " FROM ") {
				panic("new statement requires reviewed relation privileges: " + key)
			}
			matches := regexp.MustCompile(`([a-z_]+)\(`).FindAllStringSubmatch(sql, -1)
			var builtins []string
			for _, match := range matches {
				builtins = append(builtins, "pg_catalog."+match[1])
			}
			if len(builtins) == 0 {
				panic("clock statement has no functions")
			}
			id := strings.Split(literal(g.entries[0].Args[0]), ".")[0] + "." + strings.TrimPrefix(builtins[0], "pg_catalog.")
			entry := &ast.CallExpr{Fun: ast.NewIdent("statement"), Args: []ast.Expr{quoted(id), g.path, quoted(g.fn.Name.Name), &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(i)}, quoted("SELECT"), ast.NewIdent("nil"), quoted(strings.Join(builtins, " "))}}
			statements.Elts = append(statements.Elts, entry)
		}
		for _, entry := range g.entries {
			index, err := strconv.Atoi(entry.Args[3].(*ast.BasicLit).Value)
			must(err)
			index += missing
			if index >= len(sqlCalls) {
				panic("SQL call absent: " + key)
			}
			entry.Args[2] = quoted(g.fn.Name.Name)
			entry.Args[3] = &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(index)}
		}
	}
	var out bytes.Buffer
	must(format.Node(&out, fset, file))
	raw := bytes.ReplaceAll(out.Bytes(), []byte("), statement("), []byte("),\n\tstatement("))
	raw, err = format.Source(raw)
	must(err)
	must(os.WriteFile(path, raw, 0644))
	fmt.Println("Updated runtime SQL statement locations and clock privileges")
}
