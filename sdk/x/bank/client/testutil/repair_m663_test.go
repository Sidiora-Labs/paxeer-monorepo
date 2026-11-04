package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestRepairM_663_BankCLINetworkSignature(t *testing.T) {
	entry, err := parser.ParseFile(token.NewFileSet(), "cli_test.go", nil, 0)
	require.NoError(t, err)
	var calls int
	for _, decl := range entry.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "TestIntegrationTestSuite" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
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
			calls++
			require.Len(t, call.Args, 1)
			handle, ok := call.Args[0].(*ast.Ident)
			require.True(t, ok)
			require.Equal(t, "t", handle.Name)
			return true
		})
	}
	require.Equal(t, 1, calls)

	cfg := network.DefaultConfig(t)
	cfg.NumValidators = 1
	suite.Run(t, NewIntegrationTestSuite(cfg))
}
