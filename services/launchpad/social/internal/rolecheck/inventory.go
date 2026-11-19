// Package rolecheck describes the SQL used by the social runtime.
package rolecheck

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const RuntimeRole = "chat_runtime"

const socialSchema = "chat"

// Statement identifies a runtime SQL call and the objects its text names.
type Statement struct {
	ID        string            `json:"id"`
	File      string            `json:"file"`
	Function  string            `json:"function"`
	Ordinal   int               `json:"ordinal"`
	Role      string            `json:"role"`
	Kind      string            `json:"kind"`
	Tables    []string          `json:"tables"`
	Sequences []string          `json:"sequences"`
	Functions []string          `json:"functions"`
	Columns   []string          `json:"columns"`
	Bindings  map[string]string `json:"bindings"`
	SQL       string            `json:"sql"`
}

//go:embed statements.json
var statementsJSON []byte

// Statements returns the SQL inventory, checked against the service's source.
func Statements(root string) ([]Statement, error) {
	var statements []Statement
	if err := json.Unmarshal(statementsJSON, &statements); err != nil {
		return nil, err
	}
	sources := &sourceReader{root: root, fset: token.NewFileSet(), files: map[string]*ast.File{}}
	for _, s := range statements {
		file, err := sources.read(s.File)
		if err != nil {
			return nil, err
		}
		var function *ast.FuncDecl
		for _, declaration := range file.Decls {
			if fn, ok := declaration.(*ast.FuncDecl); ok && functionName(fn) == s.Function {
				function = fn
				break
			}
		}
		if function == nil {
			return nil, fmt.Errorf("%s: function absent", s.ID)
		}
		calls := queryCalls(function)
		if s.Ordinal < 1 || s.Ordinal > len(calls) {
			return nil, fmt.Errorf("%s: SQL call absent", s.ID)
		}
		call := calls[s.Ordinal-1]
		sql, err := sources.evaluate(call.Args[1], s.File, function, call.Pos(), s.Bindings, 0)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.ID, err)
		}
		if sql != s.SQL {
			return nil, fmt.Errorf("%s: SQL differs from source", s.ID)
		}
	}
	return statements, nil
}

type sourceReader struct {
	root  string
	fset  *token.FileSet
	files map[string]*ast.File
}

func (r *sourceReader) read(path string) (*ast.File, error) {
	if f := r.files[path]; f != nil {
		return f, nil
	}
	f, err := parser.ParseFile(r.fset, filepath.Join(r.root, path), nil, 0)
	if err != nil {
		return nil, err
	}
	r.files[path] = f
	return f, nil
}

func functionName(fn *ast.FuncDecl) string {
	name := fn.Name.Name
	if fn.Recv != nil {
		receiver := fn.Recv.List[0].Type
		if pointer, ok := receiver.(*ast.StarExpr); ok {
			receiver = pointer.X
		}
		name = receiver.(*ast.Ident).Name + "." + name
	}
	return name
}

func queryCalls(node ast.Node) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "Query", "QueryRow", "Exec", "Prepare", "CopyFrom", "SendBatch":
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func expressionName(node ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), node); err != nil {
		return ""
	}
	return b.String()
}

func (r *sourceReader) evaluate(expr ast.Expr, path string, fn *ast.FuncDecl, before token.Pos, bindings map[string]string, depth int) (string, error) {
	if depth > 30 {
		return "", fmt.Errorf("SQL expression recursion")
	}
	next := func(e ast.Expr, pos token.Pos) (string, error) {
		return r.evaluate(e, path, fn, pos, bindings, depth+1)
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strconv.Unquote(e.Value)
		}
	case *ast.ParenExpr:
		return next(e.X, before)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			break
		}
		left, err := next(e.X, before)
		if err != nil {
			return "", err
		}
		right, err := next(e.Y, before)
		return left + right, err
	case *ast.Ident:
		if value, ok := bindings[e.Name]; ok {
			return value, nil
		}
		var value ast.Expr
		ast.Inspect(fn, func(node ast.Node) bool {
			if node == nil || node.Pos() >= before {
				return false
			}
			switch assignment := node.(type) {
			case *ast.AssignStmt:
				if len(assignment.Lhs) == len(assignment.Rhs) {
					for i, lhs := range assignment.Lhs {
						if name, ok := lhs.(*ast.Ident); ok && name.Name == e.Name {
							value = assignment.Rhs[i]
						}
					}
				}
			case *ast.ValueSpec:
				if len(assignment.Names) == len(assignment.Values) {
					for i, name := range assignment.Names {
						if name.Name == e.Name {
							value = assignment.Values[i]
						}
					}
				}
			}
			return true
		})
		if value != nil {
			return next(value, value.Pos())
		}
		siblings, err := os.ReadDir(filepath.Join(r.root, filepath.Dir(path)))
		if err != nil {
			return "", err
		}
		for _, sibling := range siblings {
			if !strings.HasSuffix(sibling.Name(), ".go") || strings.HasSuffix(sibling.Name(), "_test.go") {
				continue
			}
			siblingPath := filepath.Join(filepath.Dir(path), sibling.Name())
			file, err := r.read(siblingPath)
			if err != nil {
				return "", err
			}
			for _, declaration := range file.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range general.Specs {
					v, ok := spec.(*ast.ValueSpec)
					if !ok || len(v.Names) != len(v.Values) {
						continue
					}
					for i, name := range v.Names {
						if name.Name == e.Name {
							return r.evaluate(v.Values[i], siblingPath, fn, before, bindings, depth+1)
						}
					}
				}
			}
		}
	case *ast.SelectorExpr:
		if expressionName(e) == "g.schema" {
			return pgx.Identifier{"recovery"}.Sanitize(), nil
		}
	case *ast.CallExpr:
		switch expressionName(e.Fun) {
		case "s.table":
			name, err := next(e.Args[0], before)
			return pgx.Identifier{socialSchema}.Sanitize() + "." + name, err
		case "fmt.Sprintf":
			template, err := next(e.Args[0], before)
			if err != nil {
				return "", err
			}
			args := make([]any, 0, len(e.Args)-1)
			for _, argument := range e.Args[1:] {
				value, err := next(argument, before)
				if err != nil {
					return "", err
				}
				args = append(args, value)
			}
			return fmt.Sprintf(template, args...), nil
		case "p.Guard.Sanitize", "p.PoolGuard.Sanitize":
			return pgx.Identifier{socialSchema, "interaction_pool_creator"}.Sanitize(), nil
		case "p.SessionGuard.Sanitize":
			return pgx.Identifier{socialSchema, "interaction_session_valid"}.Sanitize(), nil
		case "p.AccountGuard.Sanitize":
			return pgx.Identifier{socialSchema, "interaction_account_active"}.Sanitize(), nil
		case "p.Accounts.Sanitize":
			return pgx.Identifier{"users", "account_profiles_v1"}.Sanitize(), nil
		}
	}
	return "", fmt.Errorf("unsupported SQL expression %s", expressionName(expr))
}

// Packages returns the number of statement variants in each source package.
func Packages(statements []Statement) map[string]int {
	counts := map[string]int{}
	for _, statement := range statements {
		counts[filepath.Dir(statement.File)]++
	}
	return counts
}

func packageNames(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
