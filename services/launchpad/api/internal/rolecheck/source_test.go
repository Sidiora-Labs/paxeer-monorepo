package rolecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var sqlStart = regexp.MustCompile(`(?i)^\s*(SELECT|INSERT|UPDATE|DELETE|WITH|SET LOCAL)\s`)

type sourceFile struct {
	files   *token.FileSet
	file    *ast.File
	raw     map[string][]byte
	helpers map[string]*ast.FuncDecl
	globals map[string]ast.Expr
}

func openSource(root, path string) (*sourceFile, error) {
	files := token.NewFileSet()
	dir := filepath.Join(root, filepath.Dir(path))
	packages, err := parser.ParseDir(files, dir, func(info os.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, 0)
	if err != nil {
		return nil, err
	}
	source := &sourceFile{files: files, globals: map[string]ast.Expr{}, raw: map[string][]byte{}, helpers: map[string]*ast.FuncDecl{}}
	for _, pkg := range packages {
		for name, file := range pkg.Files {
			if filepath.Clean(name) == filepath.Join(root, path) {
				source.file = file
			}
			source.raw[name], err = os.ReadFile(name)
			if err != nil {
				return nil, err
			}
			for _, declaration := range file.Decls {
				if fn, ok := declaration.(*ast.FuncDecl); ok {
					source.helpers[functionName(fn)] = fn
				}
				if gen, ok := declaration.(*ast.GenDecl); ok {
					for _, spec := range gen.Specs {
						if value, ok := spec.(*ast.ValueSpec); ok && len(value.Names) == len(value.Values) {
							for i, name := range value.Names {
								source.globals[name.Name] = value.Values[i]
							}
						}
					}
				}
			}
		}
	}
	if source.file == nil {
		return nil, fmt.Errorf("source file absent: %s", path)
	}
	return source, nil
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

func (s *sourceFile) text(node ast.Node) string {
	pos := s.files.Position(node.Pos())
	return string(s.raw[pos.Filename][pos.Offset:s.files.Position(node.End()).Offset])
}

func (s *sourceFile) function(name string) *ast.FuncDecl {
	for _, declaration := range s.file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && functionName(fn) == name {
			return fn
		}
	}
	return nil
}

func (s *sourceFile) definitions(fn *ast.FuncDecl) map[string]ast.Expr {
	definitions := make(map[string]ast.Expr, len(s.globals))
	for name, expr := range s.globals {
		definitions[name] = expr
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE && len(assign.Lhs) == len(assign.Rhs) {
			for i, lhs := range assign.Lhs {
				if name, ok := lhs.(*ast.Ident); ok {
					definitions[name.Name] = assign.Rhs[i]
				}
			}
		}
		return true
	})
	return definitions
}

func (s *sourceFile) expression(fn *ast.FuncDecl, text string) ast.Expr {
	var found ast.Expr
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if expr, ok := node.(ast.Expr); ok && s.text(expr) == text {
			found = expr
			return false
		}
		return true
	})
	return found
}

// SQL is assembled from the service's Go expression and registered schema
// identifiers. Local string definitions and the table helper are read from
// source, so changes to those fragments also change the statement under test.
func (s *sourceFile) evaluate(expr ast.Expr, definitions map[string]ast.Expr, bindings map[string]string, depth int) (string, error) {
	if depth > 32 {
		return "", fmt.Errorf("cyclic SQL expression")
	}
	evaluate := func(e ast.Expr) (string, error) { return s.evaluate(e, definitions, bindings, depth+1) }
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strconv.Unquote(e.Value)
		}
		if e.Kind == token.INT {
			return e.Value, nil
		}
	case *ast.ParenExpr:
		return evaluate(e.X)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			left, err := evaluate(e.X)
			if err != nil {
				return "", err
			}
			right, err := evaluate(e.Y)
			if err != nil {
				return "", err
			}
			return left + right, nil
		}
	case *ast.Ident:
		if definition, ok := definitions[e.Name]; ok {
			return evaluate(definition)
		}
		if value, ok := bindings[e.Name]; ok {
			return value, nil
		}
	case *ast.SelectorExpr:
		if value, ok := bindings[s.text(e)]; ok {
			return value, nil
		}
	case *ast.CallExpr:
		if selector, ok := e.Fun.(*ast.SelectorExpr); ok {
			if selector.Sel.Name == "Sanitize" && len(e.Args) == 0 {
				if literal, ok := selector.X.(*ast.CompositeLit); ok {
					typ, valid := literal.Type.(*ast.SelectorExpr)
					if !valid || typ.Sel.Name != "Identifier" {
						break
					}
					parts := make([]string, 0, len(literal.Elts))
					for _, element := range literal.Elts {
						value, err := evaluate(element)
						if err != nil {
							return "", err
						}
						parts = append(parts, `"`+strings.ReplaceAll(value, `"`, `""`)+`"`)
					}
					return strings.Join(parts, "."), nil
				}
			}
			if selector.Sel.Name == "table" && len(e.Args) == 1 {
				helper := s.helpers["PostgresStream.table"]
				if helper == nil {
					return "", fmt.Errorf("stream table helper absent")
				}
				name, err := evaluate(e.Args[0])
				if err != nil {
					return "", err
				}
				local := make(map[string]string, len(bindings)+1)
				for key, value := range bindings {
					local[key] = value
				}
				local["name"] = name
				for _, statement := range helper.Body.List {
					if result, ok := statement.(*ast.ReturnStmt); ok && len(result.Results) == 1 {
						return s.evaluate(result.Results[0], s.definitions(helper), local, depth+1)
					}
				}
			}
			if s.text(e.Fun) == "fmt.Sprintf" && len(e.Args) > 0 {
				format, err := evaluate(e.Args[0])
				if err != nil {
					return "", err
				}
				args := make([]any, 0, len(e.Args)-1)
				for _, arg := range e.Args[1:] {
					value, err := evaluate(arg)
					if err != nil {
						return "", err
					}
					args = append(args, value)
				}
				return fmt.Sprintf(format, args...), nil
			}
		}
	}
	return "", fmt.Errorf("unbound SQL expression: %s", s.text(expr))
}

func (s *sourceFile) sqlExpressions(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	var stack []ast.Node
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			value, err := strconv.Unquote(literal.Value)
			if err == nil && sqlStart.MatchString(value) && !strings.HasPrefix(value, "DELETE /") {
				var root ast.Node = literal
				for i := len(stack) - 1; i >= 0; i-- {
					parent := stack[i]
					switch p := parent.(type) {
					case *ast.BinaryExpr:
						root = parent
					case *ast.ParenExpr:
						root = parent
					case *ast.CallExpr:
						if s.text(p.Fun) == "fmt.Sprintf" {
							root = parent
						}
						i = -1
					default:
						i = -1
					}
				}
				out[s.text(root)] = true
			}
		}
		stack = append(stack, node)
		return true
	})
	return out
}
