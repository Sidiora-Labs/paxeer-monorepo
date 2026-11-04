package testutil

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant/keeper"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func TestRepairM_680_TestConfig(t *testing.T) {
	// The integration entry point has a norace build tag, so inspect it as well
	// as exercising its real configuration and codec below.
	file, err := parser.ParseFile(token.NewFileSet(), "cli_test.go", nil, 0)
	require.NoError(t, err)
	calls := 0
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
		require.Len(t, call.Args, 1)
		arg, ok := call.Args[0].(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "t", arg.Name)
		return true
	})
	require.Equal(t, 1, calls)

	cfg := network.DefaultConfig(t)
	integration := NewIntegrationTestSuite(cfg)
	require.NotNil(t, cfg.AppConstructor)
	require.NotEmpty(t, cfg.GenesisState[feegrant.ModuleName])
	var genesis feegrant.GenesisState
	require.NoError(t, integration.cfg.Codec.UnmarshalAsJSON(cfg.GenesisState[feegrant.ModuleName], &genesis))
	require.NoError(t, feegrant.ValidateGenesis(genesis))
	filtered, err := feegrant.NewAllowedMsgAllowance(
		&feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))},
		[]string{sdk.MsgTypeURL(&govtypes.MsgSubmitProposal{})},
	)
	require.NoError(t, err)
	for _, allowance := range []feegrant.FeeAllowanceI{
		&feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))},
		&feegrant.PeriodicAllowance{
			Basic:  feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))},
			Period: time.Hour, PeriodSpendLimit: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10)),
		},
		filtered,
	} {
		encoded, err := cfg.Codec.MarshalInterface(allowance.(proto.Message))
		require.NoError(t, err)
		var decoded feegrant.FeeAllowanceI
		require.NoError(t, cfg.Codec.UnmarshalInterface(encoded, &decoded))
		require.Equal(t, allowance, decoded)
	}
}

func TestRepairM_680_FeeGrantCLIFixtures(t *testing.T) {
	cfg := network.DefaultConfig(t)
	kb := keyring.NewInMemory(cfg.KeyringOptions...)
	clientCtx := client.Context{}.WithCodec(cfg.Codec).WithInterfaceRegistry(cfg.InterfaceRegistry).
		WithLegacyAmino(cfg.LegacyAmino).WithTxConfig(cfg.TxConfig).WithKeyring(kb).
		WithAccountRetriever(cfg.AccountRetriever).WithChainID(cfg.ChainID).
		WithGenerateOnly(true).WithOffline(true)
	s := NewIntegrationTestSuite(cfg)
	s.SetT(t)
	s.network = &network.Network{Validators: []*network.Validator{{ClientCtx: clientCtx}}}
	granter := s.fixtureAddress("granter")
	granterInfo, err := kb.KeyByAddress(granter)
	require.NoError(t, err)

	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	multi := store.NewCommitMultiStore(db)
	feeKey := sdk.NewKVStoreKey(feegrant.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	paramsTransientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	multi.MountStoreWithDB(feeKey, sdk.StoreTypeIAVL, db)
	multi.MountStoreWithDB(authKey, sdk.StoreTypeIAVL, db)
	multi.MountStoreWithDB(paramsKey, sdk.StoreTypeIAVL, db)
	multi.MountStoreWithDB(paramsTransientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, multi.LoadLatestVersion())
	ctx := sdk.NewContext(multi, tmproto.Header{Time: time.Now().UTC()}, false)
	params := paramtypes.NewSubspace(cfg.Codec, cfg.LegacyAmino, paramsKey, paramsTransientKey, authtypes.ModuleName)
	accounts := authkeeper.NewAccountKeeper(cfg.Codec, authKey, params, authtypes.ProtoBaseAccount, nil)
	k := keeper.NewKeeper(cfg.Codec, feeKey, accounts)
	messages := keeper.NewMsgServerImpl(k)
	queryRouter := baseapp.NewQueryServerTestHelper(ctx, cfg.InterfaceRegistry)
	feegrant.RegisterQueryServer(queryRouter, k)
	queries := feegrant.NewQueryClient(queryRouter)

	testCases := []struct {
		name       string
		flags      []string
		useKeyName bool
		periodic   bool
		filtered   bool
		limited    bool
	}{
		{name: "basic", flags: []string{"--spend-limit=100uhpx"}, limited: true},
		{name: "key-name", flags: []string{"--spend-limit=100uhpx"}, useKeyName: true, limited: true},
		{name: "amino", flags: []string{"--spend-limit=100uhpx", "--sign-mode=amino-json"}, limited: true},
		{name: "basic-unlimited"},
		{name: "basic-expiration", flags: []string{"--expiration=" + getFormattedExpiration(oneYear)}},
		{name: "periodic", flags: []string{"--spend-limit=100uhpx", "--period=3600", "--period-limit=10uhpx"}, periodic: true, limited: true},
		{name: "periodic-unlimited", flags: []string{"--period=3600", "--period-limit=10uhpx"}, periodic: true},
		{name: "filtered", flags: []string{"--spend-limit=100uhpx", "--allowed-messages=" + sdk.MsgTypeURL(&govtypes.MsgSubmitProposal{})}, filtered: true, limited: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s.SetT(t)
			grantee := s.fixtureAddress("grantee")
			require.NotEqual(t, granter, grantee)
			for _, addr := range []sdk.AccAddress{granter, grantee} {
				require.Contains(t, addr.String(), "pax1")
				parsed, err := sdk.AccAddressFromBech32(addr.String())
				require.NoError(t, err)
				require.Equal(t, addr, parsed)
				info, err := kb.KeyByAddress(addr)
				require.NoError(t, err)
				require.Equal(t, addr, sdk.AccAddress(info.GetPubKey().Address()))
			}
			from := granter.String()
			if tc.useKeyName {
				from = granterInfo.GetName()
			}
			args := append([]string{from, grantee.String()}, tc.flags...)
			msg := repairM680CLIMessage(t, clientCtx, cli.NewCmdFeeGrant(), args).(*feegrant.MsgGrantAllowance)
			require.Equal(t, granter.String(), msg.Granter)
			require.Equal(t, grantee.String(), msg.Grantee)
			_, err := messages.GrantAllowance(sdk.WrapSDKContext(ctx), msg)
			require.NoError(t, err)
			require.NotNil(t, accounts.GetAccount(ctx, grantee))
			_, err = messages.GrantAllowance(sdk.WrapSDKContext(ctx), msg)
			require.ErrorContains(t, err, "fee allowance already exists")

			response, err := queries.Allowance(context.Background(), &feegrant.QueryAllowanceRequest{Granter: granter.String(), Grantee: grantee.String()})
			require.NoError(t, err)
			json, err := cfg.Codec.MarshalAsJSON(response.Allowance)
			require.NoError(t, err)
			var actual feegrant.Grant
			require.NoError(t, cfg.Codec.UnmarshalAsJSON(json, &actual))
			require.Equal(t, msg.Granter, actual.Granter)
			require.Equal(t, msg.Grantee, actual.Grantee)
			allowance, err := actual.GetGrant()
			require.NoError(t, err)
			if tc.filtered {
				filtered, ok := allowance.(*feegrant.AllowedMsgAllowance)
				require.True(t, ok)
				require.Equal(t, []string{sdk.MsgTypeURL(&govtypes.MsgSubmitProposal{})}, filtered.AllowedMessages)
				allowance, err = filtered.GetAllowance()
				require.NoError(t, err)
			}
			if tc.periodic {
				periodic, ok := allowance.(*feegrant.PeriodicAllowance)
				require.True(t, ok)
				require.Equal(t, time.Hour, periodic.Period)
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10)), periodic.PeriodSpendLimit)
				allowance = &periodic.Basic
			}
			basic, ok := allowance.(*feegrant.BasicAllowance)
			require.True(t, ok)
			if tc.limited {
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100)), basic.SpendLimit)
			} else {
				require.Empty(t, basic.SpendLimit)
			}

			byGrantee, err := queries.Allowances(context.Background(), &feegrant.QueryAllowancesRequest{Grantee: grantee.String()})
			require.NoError(t, err)
			require.Len(t, byGrantee.Allowances, 1)
			require.Equal(t, granter.String(), byGrantee.Allowances[0].Granter)
			require.Equal(t, grantee.String(), byGrantee.Allowances[0].Grantee)
			byGranter, err := queries.AllowancesByGranter(context.Background(), &feegrant.QueryAllowancesByGranterRequest{Granter: granter.String()})
			require.NoError(t, err)
			require.Len(t, byGranter.Allowances, 1)
			require.Equal(t, granter.String(), byGranter.Allowances[0].Granter)
			require.Equal(t, grantee.String(), byGranter.Allowances[0].Grantee)

			fee := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 3))
			msgs := []sdk.Msg{&govtypes.MsgSubmitProposal{}}
			if tc.filtered {
				err := k.UseGrantedFees(ctx, granter, grantee, fee, []sdk.Msg{&feegrant.MsgRevokeAllowance{}})
				require.Error(t, err)
			}
			require.NoError(t, k.UseGrantedFees(ctx, granter, grantee, fee, msgs))
			remaining, err := k.GetAllowance(ctx, granter, grantee)
			require.NoError(t, err)
			if tc.filtered {
				remaining, err = remaining.(*feegrant.AllowedMsgAllowance).GetAllowance()
				require.NoError(t, err)
			}
			if tc.periodic {
				periodic := remaining.(*feegrant.PeriodicAllowance)
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 7)), periodic.PeriodCanSpend)
				remaining = &periodic.Basic
			}
			if tc.limited {
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 97)), remaining.(*feegrant.BasicAllowance).SpendLimit)
			} else {
				require.Empty(t, remaining.(*feegrant.BasicAllowance).SpendLimit)
			}
			revoke := repairM680CLIMessage(t, clientCtx, cli.NewCmdRevokeFeegrant(), []string{granter.String(), grantee.String()}).(*feegrant.MsgRevokeAllowance)
			require.Equal(t, granter.String(), revoke.Granter)
			require.Equal(t, grantee.String(), revoke.Grantee)
			_, err = messages.RevokeAllowance(sdk.WrapSDKContext(ctx), revoke)
			require.NoError(t, err)
			_, err = queries.Allowance(context.Background(), &feegrant.QueryAllowanceRequest{Granter: granter.String(), Grantee: grantee.String()})
			require.ErrorContains(t, err, "fee-grant not found")
		})
	}
	s.SetT(t)
	missing := s.fixtureAddress("missing")
	_, err = queries.Allowance(context.Background(), &feegrant.QueryAllowanceRequest{Granter: missing.String(), Grantee: granter.String()})
	require.ErrorContains(t, err, "fee-grant not found")
	byGrantee, err := queries.Allowances(context.Background(), &feegrant.QueryAllowancesRequest{Grantee: missing.String()})
	require.NoError(t, err)
	require.Empty(t, byGrantee.Allowances)
	byGranter, err := queries.AllowancesByGranter(context.Background(), &feegrant.QueryAllowancesByGranterRequest{Granter: missing.String()})
	require.NoError(t, err)
	require.Empty(t, byGranter.Allowances)
}

func repairM680CLIMessage(t *testing.T, ctx client.Context, cmd *cobra.Command, args []string) sdk.Msg {
	t.Helper()
	args = append(args, "--generate-only", "--offline", "--gas=200000")
	out, err := clitestutil.ExecTestCLICmd(ctx, cmd, args)
	require.NoError(t, err, out.String())
	tx, err := ctx.TxConfig.TxJSONDecoder()(out.Bytes())
	require.NoError(t, err, out.String())
	encoded, err := ctx.TxConfig.TxEncoder()(tx)
	require.NoError(t, err)
	decoded, err := ctx.TxConfig.TxDecoder()(encoded)
	require.NoError(t, err)
	require.Len(t, decoded.GetMsgs(), 1)
	return decoded.GetMsgs()[0]
}
