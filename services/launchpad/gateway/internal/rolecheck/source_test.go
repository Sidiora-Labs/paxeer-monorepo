package rolecheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type sourceReader struct{ root string }

func functionName(fn *ast.FuncDecl) string {
	name := fn.Name.Name
	if fn.Recv != nil {
		receiver := fn.Recv.List[0].Type
		if ptr, ok := receiver.(*ast.StarExpr); ok {
			receiver = ptr.X
		}
		if id, ok := receiver.(*ast.Ident); ok {
			name = id.Name + "." + name
		}
	}
	return name
}

func sqlCalls(fn *ast.FuncDecl) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch method.Sel.Name {
		case "Exec", "Query", "QueryRow":
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func (r sourceReader) assertStatement(t *testing.T, s Statement) {
	t.Helper()
	path := filepath.Join(r.root, s.File)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if candidate, ok := decl.(*ast.FuncDecl); ok && functionName(candidate) == s.Function {
			fn = candidate
		}
	}
	if fn == nil {
		t.Fatalf("%s: source function absent", s.ID)
	}
	calls := sqlCalls(fn)
	if s.Call < 0 || s.Call >= len(calls) {
		t.Fatalf("%s: source call absent", s.ID)
	}
	expression := func(n ast.Node) string {
		return string(raw[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
	}
	sqlExpr := calls[s.Call].Args[1]
	matched := expression(sqlExpr) == s.Source
	if id, ok := sqlExpr.(*ast.Ident); ok && !matched {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok || assign.Pos() >= calls[s.Call].Pos() {
				return true
			}
			for i, left := range assign.Lhs {
				if name, ok := left.(*ast.Ident); ok && name.Name == id.Name && i < len(assign.Rhs) && expression(assign.Rhs[i]) == s.Source {
					matched = true
				}
			}
			return true
		})
	}
	if !matched {
		t.Fatalf("%s: inventory SQL expression differs from the source call", s.ID)
	}
	if s.Source == "statement" {
		value := s.Bindings["statement"]
		found := false
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				values, ok := spec.(*ast.ValueSpec)
				if !ok || len(values.Names) != 1 || values.Names[0].Name != "identityCollectionStatements" {
					continue
				}
				ast.Inspect(values, func(node ast.Node) bool {
					if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
						actual, err := strconv.Unquote(literal.Value)
						if err == nil && actual == value {
							found = true
						}
					}
					return true
				})
			}
		}
		if !found {
			t.Fatalf("%s: collection statement absent from source", s.ID)
		}
	}
	parsed, err := parser.ParseExpr(s.Source)
	if err != nil {
		t.Fatal(err)
	}
	sql := r.text(t, s, parsed)
	if strings.Contains(s.File, "/internal/auth/") || strings.Contains(s.File, "/shared/auth/") {
		sql = qualifyIdentitySQL(sql)
	}
	if sql != s.SQL {
		t.Fatalf("%s: inventory SQL differs from connected source: %s", s.ID, sql)
	}
	if strings.Fields(sql)[0] != s.Kind {
		t.Fatalf("%s: statement kind differs", s.ID)
	}
}

func (r sourceReader) text(t *testing.T, s Statement, expr ast.Expr) string {
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
	case *ast.Ident:
		if value, ok := s.Bindings[v.Name]; ok {
			return value
		}
	case *ast.SelectorExpr:
		if receiver, ok := v.X.(*ast.Ident); ok {
			if value, ok := s.Bindings[receiver.Name+"."+v.Sel.Name]; ok {
				return value
			}
		}
		if parent, ok := v.X.(*ast.SelectorExpr); ok {
			if receiver, ok := parent.X.(*ast.Ident); ok {
				if value, ok := s.Bindings[receiver.Name+"."+parent.Sel.Name+"."+v.Sel.Name]; ok {
					return value
				}
			}
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return r.text(t, s, v.X) + r.text(t, s, v.Y)
		}
	case *ast.CallExpr:
		if method, ok := v.Fun.(*ast.SelectorExpr); ok {
			switch method.Sel.Name {
			case "table", "metadata", "safety":
				if len(v.Args) != 1 {
					t.Fatalf("%s: invalid table expression", s.ID)
				}
				name := r.text(t, s, v.Args[0])
				schema := "gateway"
				switch method.Sel.Name {
				case "table":
					if strings.Contains(s.File, "/shared/delivery/") {
						schema = s.Bindings["deliverySchema"]
					}
				case "metadata":
					schema = "metadata"
					name += "_v2"
				case "safety":
					schema = "chat"
					name += "_v2"
				}
				return pgx.Identifier{schema, name}.Sanitize()
			case "currentHoldPredicate":
				path := filepath.Join(r.root, "services/launchpad/gateway/internal/storage/retention.go")
				file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				for _, decl := range file.Decls {
					if fn, ok := decl.(*ast.FuncDecl); ok && functionName(fn) == "Authority.currentHoldPredicate" {
						ret, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt)
						if ok && len(ret.Results) == 1 {
							return r.text(t, s, ret.Results[0])
						}
					}
				}
			}
		}
	}
	t.Fatalf("%s: unresolved SQL expression %T", s.ID, expr)
	return ""
}

var identityObject = regexp.MustCompile(`\b(identity_[a-z_]+|security_audit_[a-z_]+|operation_ledger)\b`)

func qualifyIdentitySQL(sql string) string {
	matches := identityObject.FindAllStringIndex(sql, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		start, end := matches[i][0], matches[i][1]
		if start > 0 && (sql[start-1] == '.' || sql[start-1] == '"') {
			continue
		}
		sql = sql[:start] + "users." + sql[start:end] + sql[end:]
	}
	return sql
}

func (r sourceReader) assertCoverage(t *testing.T) {
	t.Helper()
	represented := map[string]map[int]bool{}
	for _, s := range RuntimeRoleStatements {
		key := s.File + ":" + s.Function
		if represented[key] == nil {
			represented[key] = map[int]bool{}
		}
		represented[key][s.Call] = true
	}
	for _, directory := range []string{"services/launchpad/gateway/internal/auth", "services/launchpad/gateway/internal/storage", "services/launchpad/security/control"} {
		paths, err := filepath.Glob(filepath.Join(r.root, directory, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			name := filepath.Base(path)
			if strings.HasSuffix(name, "_test.go") || name == "identity_measurement.go" || name == "envelope.go" {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			relative, err := filepath.Rel(r.root, path)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || functionName(fn) == "Store.Install" {
					continue
				}
				key := relative + ":" + functionName(fn)
				for call := range sqlCalls(fn) {
					if !represented[key][call] {
						t.Errorf("runtime SQL absent from inventory: %s call=%d", key, call)
					}
				}
			}
		}
	}
}
