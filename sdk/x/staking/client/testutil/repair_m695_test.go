package testutil

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM695Fixture struct {
	suite    *IntegrationTestSuite
	requests chan abci.RequestQuery
}

func repairM695Network(t *testing.T, cfg network.Config) repairM695Fixture {
	t.Helper()
	db := dbm.NewMemDB()
	ms := rootmulti.NewStore(db)
	keys := sdk.NewKVStoreKeys("auth", "bank", "staking", "params")
	for _, key := range keys {
		ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	transient := sdk.NewTransientStoreKey("transient_params")
	ms.MountStoreWithDB(transient, storetypes.StoreTypeTransient, nil)
	deferred := storetypes.NewMemoryStoreKey("deferred_bank")
	ms.MountStoreWithDB(deferred, storetypes.StoreTypeMemory, nil)
	require.NoError(t, ms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, ms.Close()) })
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 1, Time: time.Unix(100, 0).UTC()}, false)
	pk := paramskeeper.NewKeeper(cfg.Codec, cfg.LegacyAmino, keys["params"], transient)
	ak := authkeeper.NewAccountKeeper(cfg.Codec, keys["auth"], pk.Subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{
			"mint":                  {authtypes.Minter},
			types.BondedPoolName:    {authtypes.Staking, authtypes.Burner},
			types.NotBondedPoolName: {authtypes.Staking, authtypes.Burner},
		})
	bk := bankkeeper.NewBaseKeeperWithDeferredCache(cfg.Codec, keys["bank"], ak, pk.Subspace("bank"), map[string]bool{}, deferred)
	sk := keeper.NewKeeper(cfg.Codec, keys["staking"], ak, bk, pk.Subspace("staking"))
	ak.SetParams(ctx, authtypes.DefaultParams())
	bk.SetParams(ctx, banktypes.DefaultParams())
	var genesis types.GenesisState
	require.NoError(t, cfg.Codec.UnmarshalAsJSON(cfg.GenesisState[types.ModuleName], &genesis))
	require.NoError(t, genesis.Params.Validate())
	sk.SetParams(ctx, genesis.Params)
	for _, name := range []string{"mint", types.BondedPoolName, types.NotBondedPoolName} {
		ak.GetModuleAccount(ctx, name)
	}
	delegate := func(c sdk.Context, delegator sdk.AccAddress, validator sdk.ValAddress, amount sdk.Int) {
		coins := sdk.NewCoins(sdk.NewCoin(cfg.BondDenom, amount))
		require.NoError(t, bk.MintCoins(c, "mint", coins))
		require.NoError(t, bk.SendCoinsFromModuleToAccount(c, "mint", delegator, coins))
		val, found := sk.GetValidator(c, validator)
		require.True(t, found)
		shares, err := sk.Delegate(c, delegator, amount, types.Unbonded, val, true)
		require.NoError(t, err)
		require.True(t, sdk.NewDecFromInt(amount).Equal(shares))
	}
	validators := make([]*network.Validator, 2)
	for i := range validators {
		pubkey := ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair-m695-validator-%d", i))).PubKey()
		address := sdk.AccAddress(pubkey.Address())
		valAddress := sdk.ValAddress(pubkey.Address())
		val, err := types.NewValidator(valAddress, pubkey, types.NewDescription(fmt.Sprintf("validator-%d", i), "", "", "", ""))
		require.NoError(t, err)
		sk.SetValidator(ctx, val)
		delegate(ctx, address, valAddress, cli.DefaultTokens)
		validators[i] = &network.Validator{Address: address, ValAddress: valAddress}
	}
	// Keep height one's self-delegations separate from the later cross-delegation.
	historical, _ := ctx.CacheContext()
	current, _ := ctx.CacheContext()
	current = current.WithBlockHeight(3)
	delegate(current, validators[0].Address, validators[1].ValAddress, sdk.NewInt(10))
	router := baseapp.NewQueryServerTestHelper(current, cfg.InterfaceRegistry)
	types.RegisterQueryServer(router, keeper.Querier{Keeper: sk})
	requests := make(chan abci.RequestQuery, 32)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			queryCtx := current
			if req.Height == 1 {
				queryCtx = historical
			} else if req.Height != 0 && req.Height != 3 {
				return nil, fmt.Errorf("unavailable fixture height %d", req.Height)
			}
			handler := router.Route(req.Path)
			if handler == nil {
				return nil, fmt.Errorf("unregistered query %q", req.Path)
			}
			request := abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove}
			requests <- request
			response, err := handler(queryCtx, request)
			if err != nil {
				return nil, err
			}
			response.Height = queryCtx.BlockHeight()
			return &coretypes.ResultABCIQuery{Response: response}, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	rpc, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithCodec(cfg.Codec).WithLegacyAmino(cfg.LegacyAmino).
		WithInterfaceRegistry(cfg.InterfaceRegistry).WithClient(rpc).WithOutputFormat("json")
	for _, validator := range validators {
		validator.ClientCtx = clientCtx
	}
	s := NewIntegrationTestSuite(cfg)
	s.SetT(t)
	s.network = &network.Network{T: t, Config: cfg, Validators: validators}
	return repairM695Fixture{suite: s, requests: requests}
}

func TestRepairM_695_StakingCLIConfig(t *testing.T) {
	// The owning entry point is tagged norace; inspect its call without launching
	// a subprocess or broadening this task's declared verification command.
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
		owner, ok := selector.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "network", owner.Name)
		require.Len(t, call.Args, 1)
		argument, ok := call.Args[0].(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "t", argument.Name)
		calls++
		return true
	})
	require.Equal(t, 1, calls)
	cfg := network.DefaultConfig(t)
	cfg.NumValidators = 2
	require.NotNil(t, cfg.AppConstructor)
	f := repairM695Network(t, cfg)
	f.suite.TestGetCmdQueryParams()
	require.Len(t, f.requests, 2)
	for len(f.requests) > 0 {
		require.Equal(t, "/cosmos.staking.v1beta1.Query/Params", (<-f.requests).Path)
	}
}

func TestRepairM_695_WrongValidatorCLIQuery(t *testing.T) {
	f := repairM695Network(t, network.DefaultConfig(t))
	f.suite.TestGetCmdQueryValidatorDelegations()
	require.Len(t, f.requests, 3)
	historical, current, account := <-f.requests, <-f.requests, <-f.requests
	require.Equal(t, "/cosmos.staking.v1beta1.Query/ValidatorDelegations", historical.Path)
	require.Equal(t, int64(1), historical.Height)
	require.Equal(t, historical.Path, current.Path)
	require.Zero(t, current.Height)
	require.Equal(t, "/cosmos.staking.v1beta1.Query/DelegatorDelegations", account.Path)
	var validatorRequest types.QueryValidatorDelegationsRequest
	require.NoError(t, f.suite.cfg.Codec.Unmarshal(current.Data, &validatorRequest))
	require.Equal(t, f.suite.network.Validators[1].ValAddress.String(), validatorRequest.ValidatorAddr)
	var accountRequest types.QueryDelegatorDelegationsRequest
	require.NoError(t, f.suite.cfg.Codec.Unmarshal(account.Data, &accountRequest))
	require.Equal(t, f.suite.network.Validators[1].Address.String(), accountRequest.DelegatorAddr)
}

func TestRepairM_695_CLIParamsFixture(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom=%t", custom), func(t *testing.T) {
			cfg := network.DefaultConfig(t)
			var genesis types.GenesisState
			require.NoError(t, cfg.Codec.UnmarshalAsJSON(cfg.GenesisState[types.ModuleName], &genesis))
			if custom {
				genesis.Params = types.NewParams(9*time.Hour, 19, 11, 42, "urepair", sdk.MustNewDecFromStr("0.13"), sdk.MustNewDecFromStr("0.37"), sdk.NewInt(2000000))
				cfg.BondDenom = genesis.Params.BondDenom
				cfg.GenesisState[types.ModuleName] = cfg.Codec.MustMarshalJSON(&genesis)
			}
			f := repairM695Network(t, cfg)
			f.suite.TestGetCmdQueryParams()
			require.Len(t, f.requests, 2)
			if !custom {
				ctx := f.suite.network.Validators[0].ClientCtx
				out, err := clitestutil.ExecTestCLICmd(ctx, cli.GetCmdQueryParams(), []string{fmt.Sprintf("--%s=json", flags.FlagOutput)})
				require.NoError(t, err)
				require.JSONEq(t, `{"unbonding_time":"1814400s","max_validators":35,"max_entries":7,"historical_entries":10000,"bond_denom":"uhpx","min_commission_rate":"0.050000000000000000","max_voting_power_ratio":"0.200000000000000000","max_voting_power_enforcement_threshold":"1000000"}`, out.String())
				out, err = clitestutil.ExecTestCLICmd(ctx, cli.GetCmdQueryParams(), []string{fmt.Sprintf("--%s=text", flags.FlagOutput)})
				require.NoError(t, err)
				require.Equal(t, "bond_denom: uhpx\nhistorical_entries: 10000\nmax_entries: 7\nmax_validators: 35\nmax_voting_power_enforcement_threshold: \"1000000\"\nmax_voting_power_ratio: \"0.200000000000000000\"\nmin_commission_rate: \"0.050000000000000000\"\nunbonding_time: 1814400s\n", out.String())
			}
		})
	}
}
