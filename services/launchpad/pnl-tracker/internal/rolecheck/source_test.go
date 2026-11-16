package rolecheck

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type sourceFile struct {
	path string
	raw  []byte
	ast  *ast.File
}

type sourceIndex struct {
	root      string
	positions *token.FileSet
	files     map[string]*sourceFile
	schema    string
}

func (s *sourceIndex) file(path string) (*sourceFile, error) {
	if f := s.files[path]; f != nil {
		return f, nil
	}
	raw, err := os.ReadFile(filepath.Join(s.root, path))
	if err != nil {
		return nil, err
	}
	parsed, err := parser.ParseFile(s.positions, path, raw, 0)
	if err != nil {
		return nil, err
	}
	f := &sourceFile{path: path, raw: raw, ast: parsed}
	s.files[path] = f
	return f, nil
}

func functionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil {
		return fn.Name.Name
	}
	typ := fn.Recv.List[0].Type
	if pointer, ok := typ.(*ast.StarExpr); ok {
		typ = pointer.X
	}
	if name, ok := typ.(*ast.Ident); ok {
		return name.Name + "." + fn.Name.Name
	}
	return ""
}

func statementCalls(fn *ast.FuncDecl) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "Exec", "Query", "QueryRow", "CopyFrom":
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func (s *sourceIndex) function(f *sourceFile, name string) (*ast.FuncDecl, error) {
	for _, declaration := range f.ast.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && functionName(fn) == name {
			return fn, nil
		}
	}
	return nil, fmt.Errorf("%s: function %s is absent", f.path, name)
}

func (s *sourceIndex) text(f *sourceFile, node ast.Node) string {
	file := s.positions.File(node.Pos())
	return string(f.raw[file.Offset(node.Pos()):file.Offset(node.End())])
}

func (s *sourceIndex) value(f *sourceFile, fn *ast.FuncDecl, before token.Pos, expression ast.Expr, bindings map[string]string, appends bool, depth int) (string, error) {
	if depth > 64 {
		return "", fmt.Errorf("SQL expression exceeds the source nesting bound")
	}
	text := s.text(f, expression)
	if value, ok := bindings[text]; ok {
		return value, nil
	}
	next := func(e ast.Expr) (string, error) { return s.value(f, fn, before, e, bindings, appends, depth+1) }
	switch node := expression.(type) {
	case *ast.BasicLit:
		if node.Kind == token.STRING {
			return strconv.Unquote(node.Value)
		}
		if node.Kind == token.INT {
			return strings.ReplaceAll(node.Value, "_", ""), nil
		}
	case *ast.ParenExpr:
		return next(node.X)
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			break
		}
		left, err := next(node.X)
		if err != nil {
			return "", err
		}
		right, err := next(node.Y)
		return left + right, err
	case *ast.CompositeLit:
		var names []string
		for _, element := range node.Elts {
			name, err := next(element)
			if err != nil {
				return "", err
			}
			names = append(names, name)
		}
		return pgx.Identifier(names).Sanitize(), nil
	case *ast.Ident:
		var initializer ast.Expr
		var additions []ast.Expr
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			assignment, ok := n.(*ast.AssignStmt)
			if !ok || assignment.Pos() >= before || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
				return true
			}
			name, ok := assignment.Lhs[0].(*ast.Ident)
			if !ok || name.Name != node.Name {
				return true
			}
			if assignment.Tok == token.DEFINE || assignment.Tok == token.ASSIGN {
				initializer = assignment.Rhs[0]
				additions = nil
			}
			if assignment.Tok == token.ADD_ASSIGN {
				additions = append(additions, assignment.Rhs[0])
			}
			return true
		})
		if initializer != nil {
			value, err := next(initializer)
			if err != nil {
				return "", err
			}
			if appends {
				for _, addition := range additions {
					suffix, err := next(addition)
					if err != nil {
						return "", err
					}
					value += suffix
				}
			}
			return value, nil
		}
		paths, err := filepath.Glob(filepath.Join(s.root, filepath.Dir(f.path), "*.go"))
		if err != nil {
			return "", err
		}
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			relative, err := filepath.Rel(s.root, path)
			if err != nil {
				return "", err
			}
			other, err := s.file(relative)
			if err != nil {
				return "", err
			}
			for _, declaration := range other.ast.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.CONST {
					continue
				}
				for _, specification := range general.Specs {
					values, ok := specification.(*ast.ValueSpec)
					if !ok || len(values.Names) != 1 || len(values.Values) != 1 || values.Names[0].Name != node.Name {
						continue
					}
					return s.value(other, fn, before, values.Values[0], bindings, appends, depth+1)
				}
			}
		}
	case *ast.CallExpr:
		if selector, ok := node.Fun.(*ast.SelectorExpr); ok {
			if selector.Sel.Name == "table" && len(node.Args) == 1 {
				name, err := next(node.Args[0])
				if err != nil {
					return "", err
				}
				// Each runtime table helper concatenates its admitted schema and table name.
				valid := false
				paths, err := filepath.Glob(filepath.Join(s.root, filepath.Dir(f.path), "*.go"))
				if err != nil {
					return "", err
				}
				for _, path := range paths {
					if strings.HasSuffix(path, "_test.go") {
						continue
					}
					relative, err := filepath.Rel(s.root, path)
					if err != nil {
						return "", err
					}
					source, err := s.file(relative)
					if err != nil {
						return "", err
					}
					for _, declaration := range source.ast.Decls {
						method, ok := declaration.(*ast.FuncDecl)
						if !ok || method.Recv == nil || method.Name.Name != "table" {
							continue
						}
						if len(method.Body.List) != 1 {
							continue
						}
						ret, ok := method.Body.List[0].(*ast.ReturnStmt)
						if !ok || len(ret.Results) != 1 {
							continue
						}
						rendered := strings.Join(strings.Fields(s.text(source, ret.Results[0])), "")
						receiver := method.Recv.List[0].Names[0].Name
						parameter := method.Type.Params.List[0].Names[0].Name
						if rendered == receiver+".schema+\".\"+"+parameter {
							valid = true
						}
					}
				}
				if !valid {
					return "", fmt.Errorf("%s: table helper requires its schema binding", f.path)
				}
				return pgx.Identifier{s.schema}.Sanitize() + "." + name, nil
			}
			if selector.Sel.Name == "Sanitize" && len(node.Args) == 0 {
				return next(selector.X)
			}
			if s.text(f, node.Fun) == "strconv.Itoa" && len(node.Args) == 1 {
				return next(node.Args[0])
			}
			if s.text(f, node.Fun) == "fmt.Sprintf" && len(node.Args) > 0 {
				format, err := next(node.Args[0])
				if err != nil {
					return "", err
				}
				var values []string
				for _, arg := range node.Args[1:] {
					value, err := next(arg)
					if err != nil {
						return "", err
					}
					values = append(values, value)
				}
				substitutions := regexp.MustCompile(`%[sd]`)
				index := 0
				result := substitutions.ReplaceAllStringFunc(format, func(string) string {
					if index >= len(values) {
						return "%!missing"
					}
					value := values[index]
					index++
					return value
				})
				if index != len(values) || strings.Contains(result, "%!missing") {
					return "", fmt.Errorf("SQL format arguments do not match")
				}
				return result, nil
			}
		}
		if name, ok := node.Fun.(*ast.Ident); ok && name.Name == "baselineCandidatesQuery" {
			helper, err := s.function(f, name.Name)
			if err != nil {
				return "", err
			}
			for _, statement := range helper.Body.List {
				if ret, ok := statement.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
					return s.value(f, helper, ret.Pos(), ret.Results[0], bindings, false, depth+1)
				}
			}
		}
	}
	return "", fmt.Errorf("%s %s: SQL expression %q requires a binding", f.path, functionName(fn), text)
}

func (s *sourceIndex) sql(statement Statement) (string, error) {
	f, err := s.file(statement.File)
	if err != nil {
		return "", err
	}
	fn, err := s.function(f, statement.Function)
	if err != nil {
		return "", err
	}
	calls := statementCalls(fn)
	if statement.Call < 1 || statement.Call > len(calls) {
		return "", fmt.Errorf("%s: SQL call %d is absent", statement.ID, statement.Call)
	}
	call := calls[statement.Call-1]
	value, err := s.value(f, fn, call.Pos(), call.Args[1], statement.Bindings, statement.Variant == "bounded", 0)
	if err != nil {
		return "", err
	}
	if call.Fun.(*ast.SelectorExpr).Sel.Name != "CopyFrom" {
		return value, nil
	}
	if len(call.Args) != 4 {
		return "", fmt.Errorf("%s: COPY arguments are incomplete", statement.ID)
	}
	columns, ok := call.Args[2].(*ast.CompositeLit)
	if !ok {
		return "", fmt.Errorf("%s: COPY column names require a binding", statement.ID)
	}
	var names []string
	for _, expression := range columns.Elts {
		name, err := s.value(f, fn, call.Pos(), expression, statement.Bindings, false, 0)
		if err != nil {
			return "", err
		}
		names = append(names, pgx.Identifier{name}.Sanitize())
	}
	return "COPY " + value + " (" + strings.Join(names, ",") + ") FROM STDIN (FORMAT BINARY)", nil
}

func copyBinaryEmpty() *bytes.Reader {
	return bytes.NewReader([]byte{'P', 'G', 'C', 'O', 'P', 'Y', '\n', 0xff, '\r', '\n', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff})
}
