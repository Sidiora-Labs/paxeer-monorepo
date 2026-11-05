package precompiles

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestSetupDeclaresNoPrecompileLastUpgrade(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "setup.go", nil, 0)
	if err != nil {
		t.Fatalf("parse setup.go: %v", err)
	}
	checkName := func(name *ast.Ident) {
		if name.Name == "PrecompileLastUpgrade" {
			t.Errorf("%s: setup.go declares retired name %s", fset.Position(name.Pos()), name.Name)
		}
	}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						checkName(name)
					}
				case *ast.TypeSpec:
					checkName(spec.Name)
				}
			}
		case *ast.FuncDecl:
			if decl.Recv == nil {
				checkName(decl.Name)
			}
		}
	}
}
