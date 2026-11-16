package rolecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type sourceReader struct {
	root  string
	files map[string]*ast.File
}

func (r *sourceReader) function(t *testing.T, path, name string) *ast.FuncDecl {
	t.Helper()
	file, ok := r.files[path]
	if !ok {
		var err error
		file, err = parser.ParseFile(token.NewFileSet(), filepath.Join(r.root, path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		r.files[path] = file
	}
	var found *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			if found != nil {
				t.Fatalf("ambiguous source function %s:%s", path, name)
			}
			found = fn
		}
	}
	if found == nil || found.Body == nil {
		t.Fatalf("source function absent: %s:%s", path, name)
	}
	return found
}

func sqlCalls(fn *ast.FuncDecl) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch method.Sel.Name {
		case "Exec", "Query", "QueryRow", "CopyFrom":
			if len(call.Args) >= 2 {
				calls = append(calls, call)
			}
		}
		return true
	})
	return calls
}

func (r *sourceReader) text(t *testing.T, path string, expr ast.Expr, bindings map[string]string) string {
	t.Helper()
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			value, err := strconv.Unquote(v.Value)
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return r.text(t, path, v.X, bindings) + r.text(t, path, v.Y, bindings)
		}
	case *ast.Ident:
		if value, ok := bindings[v.Name]; ok {
			return value
		}
	case *ast.SelectorExpr:
		if receiver, ok := v.X.(*ast.Ident); ok {
			if value, ok := bindings[receiver.Name+"."+v.Sel.Name]; ok {
				return value
			}
		}
	case *ast.CallExpr:
		if method, ok := v.Fun.(*ast.SelectorExpr); ok {
			switch method.Sel.Name {
			case "table", "source", "stats":
				if len(v.Args) == 1 {
					schema := map[string]string{"table": "ranking", "source": "indexer", "stats": "stats"}[method.Sel.Name]
					return pgx.Identifier{schema, r.text(t, path, v.Args[0], bindings)}.Sanitize()
				}
			case "Sanitize":
				if id, ok := method.X.(*ast.CompositeLit); ok {
					parts := make([]string, len(id.Elts))
					for i, part := range id.Elts {
						parts[i] = r.text(t, path, part, bindings)
					}
					return pgx.Identifier(parts).Sanitize()
				}
			case "universeQuery":
				return r.returnText(t, storePath+"frozen_inputs.go", "universeQuery", bindings)
			}
		}
		if function, ok := v.Fun.(*ast.Ident); ok && function.Name == "universeQuerySQL" && len(v.Args) == 2 {
			args := map[string]string{"commits": r.text(t, path, v.Args[0], bindings), "creations": r.text(t, path, v.Args[1], bindings)}
			return r.returnText(t, storePath+"universe_features.go", "universeQuerySQL", args)
		}
	}
	t.Fatalf("unresolved SQL expression in %s: %T", path, expr)
	return ""
}

func (r *sourceReader) returnText(t *testing.T, path, name string, bindings map[string]string) string {
	t.Helper()
	fn := r.function(t, path, name)
	if len(fn.Body.List) == 1 {
		if ret, ok := fn.Body.List[0].(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
			return r.text(t, path, ret.Results[0], bindings)
		}
	}
	t.Fatalf("SQL function %s:%s must return its statement", path, name)
	return ""
}

type resolvedStatement struct {
	SQL     string
	Table   pgx.Identifier
	Columns []string
}

func (r *sourceReader) resolve(t *testing.T, s Statement) resolvedStatement {
	t.Helper()
	fn := r.function(t, s.File, s.Function)
	calls := sqlCalls(fn)
	if s.Call < 0 || s.Call >= len(calls) {
		t.Fatalf("statement %s: source call %d absent", s.ID, s.Call)
	}
	call := calls[s.Call]
	bindings := map[string]string{
		"s.schema": "ranking", "s.Schema": "api_stream", "g.schema": pgx.Identifier{"recovery"}.Sanitize(),
		"function": "append_ranking",
	}
	for key, value := range s.Bindings {
		bindings[key] = value
	}
	if s.Kind == "COPY" {
		table, ok := call.Args[1].(*ast.CompositeLit)
		if !ok || len(call.Args) < 3 {
			t.Fatalf("%s: COPY table expression absent", s.ID)
		}
		out := resolvedStatement{}
		for _, part := range table.Elts {
			out.Table = append(out.Table, r.text(t, s.File, part, bindings))
		}
		columns, ok := call.Args[2].(*ast.CompositeLit)
		if !ok {
			t.Fatalf("%s: COPY columns absent", s.ID)
		}
		for _, column := range columns.Elts {
			out.Columns = append(out.Columns, r.text(t, s.File, column, bindings))
		}
		quoted := make([]string, len(out.Columns))
		for i, column := range out.Columns {
			quoted[i] = pgx.Identifier{column}.Sanitize()
		}
		out.SQL = "COPY " + out.Table.Sanitize() + " (" + strings.Join(quoted, ",") + ") FROM STDIN BINARY"
		return out
	}
	expr := call.Args[1]
	if name, ok := expr.(*ast.Ident); ok {
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok || assign.Pos() >= call.Pos() {
				return true
			}
			for i, lhs := range assign.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name.Name && i < len(assign.Rhs) {
					bindings[id.Name] = r.text(t, s.File, assign.Rhs[i], bindings)
					found = true
				}
			}
			return true
		})
		if !found {
			t.Fatalf("%s: SQL variable %s absent", s.ID, name.Name)
		}
	}
	sql := r.text(t, s.File, expr, bindings)
	kind := strings.Fields(sql)[0]
	if kind != s.Kind && !(s.Kind == "SELECT" && kind == "WITH") {
		t.Fatalf("%s: kind %s differs from %s", s.ID, kind, s.Kind)
	}
	return resolvedStatement{SQL: sql}
}

func (r *sourceReader) assertCoverage(t *testing.T) {
	t.Helper()
	represented := map[string]map[int]bool{}
	for _, s := range Statements {
		key := s.File + ":" + s.Function
		if represented[key] == nil {
			represented[key] = map[int]bool{}
		}
		represented[key][s.Call] = true
	}
	for key, slots := range represented {
		pos := strings.LastIndex(key, ":")
		fn := r.function(t, key[:pos], key[pos+1:])
		calls := sqlCalls(fn)
		for i := range calls {
			if !slots[i] {
				t.Errorf("runtime statement absent from inventory: %s call %d", key, i)
			}
		}
		if len(slots) != len(calls) {
			t.Errorf("runtime statement count differs: %s", key)
		}
	}
	exclusions := map[string]string{
		"store.go":                         "legacy development ranking store",
		"snapshots.go":                     "Save and Read are superseded by Complete and Header in the connected service",
		"frozen_inputs.go:ExplainUniverse": "offline measurement query",
		"publication.go:DispatchOnce":      "development direct broker publication",
	}
	paths, err := filepath.Glob(filepath.Join(r.root, storePath, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file := storePath + filepath.Base(path)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || len(sqlCalls(fn)) == 0 {
				continue
			}
			if exclusions[filepath.Base(path)] != "" || exclusions[filepath.Base(path)+":"+fn.Name.Name] != "" {
				continue
			}
			if represented[file+":"+fn.Name.Name] == nil {
				t.Error(fmt.Sprintf("runtime function absent from inventory: %s:%s", file, fn.Name.Name))
			}
		}
	}
}
