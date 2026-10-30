package api_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	programstypes "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms/types"
)

func TestLayerxProgramsDescriptorParity(t *testing.T) {
	const module = "github.com/Sidiora-Labs/Paxeer-X-Network/"
	goPackage := reflect.TypeOf(programstypes.GenesisState{}).PkgPath()
	root := filepath.Join("..", "api", "layerxprograms")
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".proto" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		count++
		name := "layerxprograms/" + filepath.ToSlash(relative)
		t.Run(name, func(t *testing.T) {
			generated := registeredFile(t, name)
			source := bufFile(t, name)
			for _, difference := range compareShapes(name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
			if got := source.GetOptions().GetGoPackage(); got != goPackage {
				t.Errorf("source go_package = %q, want registration package %q", got, goPackage)
			}
			if got := generated.GetOptions().GetGoPackage(); got != goPackage {
				t.Errorf("registered go_package = %q, want registration package %q", got, goPackage)
			}

			generatedPath := filepath.Join("..", filepath.FromSlash(strings.TrimPrefix(goPackage, module)), strings.TrimSuffix(entry.Name(), ".proto")+".pb.go")
			file, err := parser.ParseFile(token.NewFileSet(), generatedPath, nil, 0)
			if err != nil {
				t.Fatalf("read generated Go registration under go_package: %v", err)
			}
			protoImport := ""
			for _, spec := range file.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatalf("decode generated Go import: %v", err)
				}
				if importPath == "github.com/gogo/protobuf/proto" {
					protoImport = "proto"
					if spec.Name != nil {
						protoImport = spec.Name.Name
					}
				}
			}
			registered := false
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Recv != nil || function.Name.Name != "init" {
					continue
				}
				ast.Inspect(function.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok || len(call.Args) != 2 {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "RegisterFile" {
						return true
					}
					owner, ok := selector.X.(*ast.Ident)
					if !ok || protoImport == "" || owner.Name != protoImport {
						return true
					}
					literal, ok := call.Args[0].(*ast.BasicLit)
					if ok && literal.Kind == token.STRING {
						registeredName, err := strconv.Unquote(literal.Value)
						if err == nil && registeredName == name {
							registered = true
						}
					}
					return true
				})
			}
			if !registered {
				t.Errorf("%s: no init registration for %q", generatedPath, name)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk layerxprograms proto sources: %v", err)
	}
	if count == 0 {
		t.Fatal("layerxprograms: no .proto sources found")
	}
}
