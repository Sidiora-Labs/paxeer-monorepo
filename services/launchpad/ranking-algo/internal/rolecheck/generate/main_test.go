package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/rolecheck"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestGeneratorPreservesRuntimeInventory(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	copyRoot := t.TempDir()
	inventoryPath := "services/launchpad/ranking-algo/internal/rolecheck/inventory.go"
	paths := map[string]bool{inventoryPath: true, "migrations/registry.json": true}
	for _, statement := range rolecheck.Statements {
		paths[statement.File] = true
	}
	for path := range paths {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(copyRoot, path)
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(filepath.Join(copyRoot, inventoryPath))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(copyRoot, filepath.Dir(inventoryPath)))
	main()
	after, err := os.ReadFile(filepath.Join(copyRoot, inventoryPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(inventoryGrants(t, before), inventoryGrants(t, after)) {
		t.Fatal("refresh changed the runtime statement privileges")
	}
	info, err := os.Stat(filepath.Join(copyRoot, inventoryPath))
	if err != nil || info.Mode().Perm() != 0644 {
		t.Fatalf("inventory source permissions: %v, %v", info, err)
	}
	main()
	again, err := os.ReadFile(filepath.Join(copyRoot, inventoryPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, again) {
		t.Fatal("refresh changed an already current inventory")
	}
}

func inventoryGrants(t *testing.T, raw []byte) []byte {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "inventory.go", raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	var grants bytes.Buffer
	ast.Inspect(file, func(node ast.Node) bool {
		if value, ok := node.(*ast.BasicLit); ok && value.Kind == token.STRING {
			grants.WriteString(literal(value))
			grants.WriteByte(0)
		}
		return true
	})
	return grants.Bytes()
}
