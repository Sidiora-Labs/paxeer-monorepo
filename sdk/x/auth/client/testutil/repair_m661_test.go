package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestRepairM_661_AuthCLINetworkSignature(t *testing.T) {
	// The normal gate does not select norace files, so inspect the entry point
	// as well as exercising its complete network-backed suite below.
	source, err := os.ReadFile("cli_test.go")
	require.NoError(t, err)
	require.Contains(t, string(source), "//go:build norace\n// +build norace\n")
	file, err := parser.ParseFile(token.NewFileSet(), "cli_test.go", source, 0)
	require.NoError(t, err)
	found := false
	for _, decl := range file.Decls {
		entry, ok := decl.(*ast.FuncDecl)
		if !ok || entry.Name.Name != "TestIntegrationTestSuite" {
			continue
		}
		ast.Inspect(entry.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "DefaultConfig" {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "network" {
				return true
			}
			require.Len(t, call.Args, 1, "DefaultConfig must receive the suite's testing handle")
			handle, ok := call.Args[0].(*ast.Ident)
			require.True(t, ok, "DefaultConfig must receive t")
			require.Equal(t, "t", handle.Name)
			found = true
			return true
		})
	}
	require.True(t, found, "the norace entry point must initialize the real network config")

	cfg := network.DefaultConfig(t)
	suite.Run(t, NewIntegrationTestSuite(cfg))
}
