package testutil

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	vestexported "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/exported"
	vestingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func TestRepairM_662_NetworkSignature(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "cli_test.go", nil, parser.ParseComments)
	require.NoError(t, err)
	var calls int
	ast.Inspect(file, func(node ast.Node) bool {
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
		require.Len(t, call.Args, 1, "the norace entry point must pass its testing handle")
		arg, ok := call.Args[0].(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "t", arg.Name)
		return true
	})
	require.Equal(t, 1, calls)
}

type repairM662Suite struct {
	*IntegrationTestSuite
}

func (s *repairM662Suite) TestNewMsgCreateVestingAccountCmd() {
	s.IntegrationTestSuite.TestNewMsgCreateVestingAccountCmd()
	clientCtx := s.network.Validators[0].ClientCtx
	for _, tc := range []struct {
		address sdk.AccAddress
		delayed bool
	}{
		{sdk.AccAddress("addr2_______________"), false},
		{sdk.AccAddress("addr3_______________"), true},
	} {
		response, err := authtypes.NewQueryClient(clientCtx).Account(context.Background(), &authtypes.QueryAccountRequest{Address: tc.address.String()})
		s.Require().NoError(err)
		var account authtypes.AccountI
		s.Require().NoError(clientCtx.InterfaceRegistry.UnpackAny(response.Account, &account))
		var vesting vestexported.VestingAccount
		if tc.delayed {
			s.Require().IsType(&vestingtypes.DelayedVestingAccount{}, account)
			vesting = account.(*vestingtypes.DelayedVestingAccount)
		} else {
			s.Require().IsType(&vestingtypes.ContinuousVestingAccount{}, account)
			vesting = account.(*vestingtypes.ContinuousVestingAccount)
		}
		amount := sdk.NewCoins(sdk.NewInt64Coin(s.cfg.BondDenom, 10))
		s.Require().Equal(amount, vesting.GetOriginalVesting())
		s.Require().Equal(int64(4070908800), vesting.GetEndTime())
		balance, err := banktypes.NewQueryClient(clientCtx).AllBalances(context.Background(), &banktypes.QueryAllBalancesRequest{Address: tc.address.String()})
		s.Require().NoError(err)
		s.Require().Equal(amount, balance.Balances)
	}
}

func TestRepairM_662_VestingCLINetwork(t *testing.T) {
	cfg := network.DefaultConfig(t)
	cfg.NumValidators = 1
	suite.Run(t, &repairM662Suite{IntegrationTestSuite: NewIntegrationTestSuite(cfg)})
}
