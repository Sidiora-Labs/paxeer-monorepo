package rolecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
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

func (r *sourceReader) file(t *testing.T, path string) *ast.File {
	t.Helper()
	if file := r.files[path]; file != nil {
		return file
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(r.root, path), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	r.files[path] = file
	return file
}

func (r *sourceReader) function(t *testing.T, path, name string) *ast.FuncDecl {
	t.Helper()
	var found *ast.FuncDecl
	for _, decl := range r.file(t, path).Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || fn.Body == nil || len(sqlCalls(fn)) == 0 {
			continue
		}
		if found != nil {
			t.Fatalf("ambiguous SQL function %s:%s", path, name)
		}
		found = fn
	}
	if found == nil {
		t.Fatalf("SQL function absent: %s:%s", path, name)
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
			case "table":
				if len(v.Args) == 1 {
					schema := "metadata"
					if strings.HasSuffix(path, "/canonical.go") {
						schema = "indexer"
					}
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
			}
		}
		if function, ok := v.Fun.(*ast.Ident); ok && function.Name == "table" && len(v.Args) == 1 {
			return pgx.Identifier{"metadata", r.text(t, path, v.Args[0], bindings)}.Sanitize()
		}
	}
	t.Fatalf("unresolved SQL expression in %s: %T", path, expr)
	return ""
}

func (r *sourceReader) resolve(t *testing.T, s Statement) string {
	t.Helper()
	calls := sqlCalls(r.function(t, s.File, s.Function))
	if s.Call < 0 || s.Call >= len(calls) {
		t.Fatalf("%s: source call %d absent", s.ID, s.Call)
	}
	sql := r.text(t, s.File, calls[s.Call].Args[1], s.Bindings)
	if s.SQL != "" && s.SQL != sql {
		t.Fatalf("%s: source SQL differs from inventory", s.ID)
	}
	kind := strings.Fields(sql)[0]
	if kind != s.Kind && !(kind == "WITH" && s.Kind == "UPDATE") {
		t.Fatalf("%s: source kind %s differs from inventory %s", s.ID, kind, s.Kind)
	}
	return sql
}

func (r *sourceReader) assertCoverage(t *testing.T) {
	t.Helper()
	represented := map[string]map[int]bool{}
	for _, s := range RuntimeStatements {
		key := s.File + ":" + s.Function
		if represented[key] == nil {
			represented[key] = map[int]bool{}
		}
		represented[key][s.Call] = true
	}
	// The identity resolver is configured with users.identity_guard_v2.
	excludedCalls := map[string]map[int]bool{
		"sdk/launchpad/shared/auth/http_authorization.go:ResolveTx": {1: true},
	}
	for key, slots := range represented {
		pos := strings.LastIndex(key, ":")
		calls := sqlCalls(r.function(t, key[:pos], key[pos+1:]))
		for i := range calls {
			if !slots[i] && !excludedCalls[key][i] {
				t.Errorf("runtime statement absent from inventory: %s call %d", key, i)
			}
		}
	}
	excludedFiles := map[string]string{
		"internal/metadata/dev_connected.go":      "development schema and projections",
		"internal/metadata/dev_v2.go":             "development canonical reads",
		"internal/metadata/legacy_publication.go": "development legacy publication",
	}
	excludedFunctions := map[string]string{
		"internal/metadata/production.go:Apply": "v1 mutation superseded by v2 and publication handlers",
	}
	service := "services/launchpad/metadata/"
	err := filepath.WalkDir(filepath.Join(r.root, service), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(filepath.Join(r.root, service), path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if relative == "internal/rolecheck" || relative == "internal/internaltest" || relative == "internal/db/sqlcdb" || relative == "internal/migrate" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || excludedFiles[relative] != "" {
			return nil
		}
		file := service + filepath.ToSlash(relative)
		for _, decl := range r.file(t, file).Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || len(sqlCalls(fn)) == 0 || excludedFunctions[relative+":"+fn.Name.Name] != "" {
				continue
			}
			if represented[file+":"+fn.Name.Name] == nil {
				t.Error(fmt.Sprintf("runtime function absent from inventory: %s:%s", file, fn.Name.Name))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
