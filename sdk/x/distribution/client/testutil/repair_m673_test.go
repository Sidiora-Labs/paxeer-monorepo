package testutil

import (
	"context"
	"fmt"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution"
	distrkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_673_TestConfig(t *testing.T) {
	// Exercise the producer with the same argument as the norace entry point.
	cfg := network.DefaultConfig(t)
	cfg.NumValidators = 1
	s := NewIntegrationTestSuite(cfg)
	require.Equal(t, cfg.BondDenom, s.cfg.BondDenom)
	require.NotNil(t, s.cfg.AppConstructor)
	require.NotNil(t, s.cfg.TxConfig)
	require.NotNil(t, s.cfg.InterfaceRegistry)
	var genesis types.GenesisState
	require.NoError(t, cfg.Codec.UnmarshalAsJSON(s.cfg.GenesisState[types.ModuleName], &genesis))
	require.Equal(t, types.DefaultParams(), genesis.Params)
	// The configuration feeds real keepers, RPC transport and the existing CLI
	// suite assertions; no application constructor substitute is used.
	repairM673Suite(t, cfg).TestGetCmdQueryParams()
}

func TestRepairM_673_DefaultFixture(t *testing.T) {
	s := repairM673Suite(t, network.DefaultConfig(t))
	response, err := types.NewQueryClient(s.network.Validators[0].ClientCtx).Params(context.Background(), &types.QueryParamsRequest{})
	require.NoError(t, err)
	defaults := types.DefaultParams()
	require.Equal(t, defaults.CommunityTax, response.Params.CommunityTax)
	require.Equal(t, defaults.BaseProposerReward, response.Params.BaseProposerReward)
	require.Equal(t, defaults.BonusProposerReward, response.Params.BonusProposerReward)
	require.Equal(t, defaults.WithdrawAddrEnabled, response.Params.WithdrawAddrEnabled)
	s.TestGetCmdQueryParams()
}

func TestRepairM_673_OutputDenom(t *testing.T) {
	for _, denom := range []string{sdk.DefaultBondDenom, "um673"} {
		t.Run(denom, func(t *testing.T) {
			cfg := network.DefaultConfig(t)
			cfg.BondDenom = denom
			s := repairM673Suite(t, cfg)
			query := types.NewQueryClient(s.network.Validators[0].ClientCtx.WithHeight(3))
			val := sdk.ValAddress(s.network.Validators[0].Address).String()
			outstanding, err := query.ValidatorOutstandingRewards(context.Background(), &types.QueryValidatorOutstandingRewardsRequest{ValidatorAddress: val})
			require.NoError(t, err)
			// Independent numeric assertion: with zero tax/proposer rates the sole
			// validator receives the entire genesis fee balance once, at height two.
			require.Equal(t, sdk.NewDecCoins(sdk.NewInt64DecCoin(denom, distributionQueryFees)), outstanding.Rewards.Rewards)
			commission, err := query.ValidatorCommission(context.Background(), &types.QueryValidatorCommissionRequest{ValidatorAddress: val})
			require.NoError(t, err)
			require.Equal(t, sdk.NewDecCoins(sdk.NewInt64DecCoin(denom, distributionQueryFees/2)), commission.Commission.Commission)
			pool, err := query.CommunityPool(context.Background(), &types.QueryCommunityPoolRequest{})
			require.NoError(t, err)
			require.Equal(t, sdk.NewDecCoins(sdk.NewInt64DecCoin(denom, distributionQueryPool)), pool.Pool)
			rewards, err := types.NewQueryClient(s.network.Validators[0].ClientCtx.WithHeight(5)).DelegationRewards(context.Background(), &types.QueryDelegationRewardsRequest{DelegatorAddress: s.network.Validators[0].Address.String(), ValidatorAddress: val})
			require.NoError(t, err)
			require.Equal(t, sdk.NewDecCoins(sdk.NewInt64DecCoin(denom, distributionQueryFees/2)), rewards.Rewards)
			// Execute every existing success/error assertion through the actual Cobra
			// commands, production RPC codecs and keeper query handlers in both formats.
			s.TestGetCmdQueryValidatorOutstandingRewards()
			s.TestGetCmdQueryValidatorCommission()
			s.TestGetCmdQueryDelegatorRewards()
			s.TestGetCmdQueryCommunityPool()
		})
	}
}

// repairM673Suite serves real, committed in-memory keeper state over loopback
// JSON-RPC. It uses the production RPC server/client and ABCI query router so
// the existing CLI tests exercise their complete query and printing path.
func repairM673Suite(t *testing.T, cfg network.Config) *IntegrationTestSuite {
	t.Helper()
	cfg.NumValidators = 1
	cfg, err := distributionQueryGenesis(cfg)
	require.NoError(t, err)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, types.StoreKey, paramtypes.StoreKey)
	transient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "repair-m673", Time: time.Unix(1_700_000_000, 0).UTC()}, false)
	pk := paramskeeper.NewKeeper(cfg.Codec, cfg.LegacyAmino, keys[paramtypes.StoreKey], transient)
	ak := authkeeper.NewAccountKeeper(cfg.Codec, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		authtypes.FeeCollectorName: nil, types.ModuleName: nil,
		stakingtypes.BondedPoolName: nil, stakingtypes.NotBondedPoolName: nil,
	})
	bk := bankkeeper.NewBaseKeeper(cfg.Codec, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)
	sk := stakingkeeper.NewKeeper(cfg.Codec, keys[stakingtypes.StoreKey], ak, bk, pk.Subspace(stakingtypes.ModuleName))
	dk := distrkeeper.NewKeeper(cfg.Codec, keys[types.StoreKey], pk.Subspace(types.ModuleName), ak, bk, sk, authtypes.FeeCollectorName, nil)
	sk.SetHooks(dk.Hooks())
	auth.InitGenesis(ctx, ak, *authtypes.DefaultGenesisState())
	var bank banktypes.GenesisState
	require.NoError(t, cfg.Codec.UnmarshalAsJSON(cfg.GenesisState[banktypes.ModuleName], &bank))
	bank.Balances = append(bank.Balances, banktypes.Balance{Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(), Coins: sdk.NewCoins(sdk.NewCoin(cfg.BondDenom, cfg.BondedTokens))})
	bk.InitGenesis(ctx, &bank)
	var distr types.GenesisState
	require.NoError(t, cfg.Codec.UnmarshalAsJSON(cfg.GenesisState[types.ModuleName], &distr))
	dk.InitGenesis(ctx, distr)
	// The committed validator/delegation fixture matches network.New's single
	// self-delegated validator, bonded stake and 50% commission.
	consensusKey := ed25519.GenPrivKeyFromSecret([]byte("distribution repair m673 validator")).PubKey()
	address := sdk.AccAddress(consensusKey.Address())
	validator, err := stakingtypes.NewValidator(sdk.ValAddress(address), consensusKey, stakingtypes.Description{Moniker: "m673"})
	require.NoError(t, err)
	validator.Tokens = cfg.BondedTokens
	validator.DelegatorShares = cfg.BondedTokens.ToDec()
	validator.Status = stakingtypes.Bonded
	validator.Commission = stakingtypes.NewCommission(sdk.NewDecWithPrec(5, 1), sdk.OneDec(), sdk.OneDec())
	stakingGenesis := stakingtypes.DefaultGenesisState()
	stakingGenesis.Params.BondDenom = cfg.BondDenom
	stakingGenesis.LastTotalPower = sdk.ZeroInt()
	stakingGenesis.Validators = []stakingtypes.Validator{validator}
	stakingGenesis.Delegations = []stakingtypes.Delegation{stakingtypes.NewDelegation(address, validator.GetOperator(), validator.DelegatorShares)}
	staking.InitGenesis(ctx, sk, ak, bk, stakingGenesis)
	consAddr, err := validator.GetConsAddr()
	require.NoError(t, err)
	votes := []abci.VoteInfo{{Validator: abci.Validator{Address: consAddr, Power: 100}, SignedLastBlock: true}}
	headers := make(map[int64]tmproto.Header)
	committed := make(map[int64]storetypes.CommitMultiStore)
	for height := int64(1); height <= 11; height++ {
		header := tmproto.Header{ChainID: "repair-m673", Height: height, Time: ctx.BlockTime().Add(time.Second), ProposerAddress: consAddr}
		ctx = ctx.WithBlockHeader(header)
		distribution.BeginBlocker(ctx, votes, dk)
		require.Equal(t, height, cms.Commit(true).Version)
		headers[height] = header
		committed[height] = repairM673CommittedStore(t, db, keys, transient)
		require.Equal(t, height, committed[height].LastCommitID().Version)
	}
	require.True(t, bk.GetAllBalances(ctx, ak.GetModuleAddress(authtypes.FeeCollectorName)).IsZero())
	router := baseapp.NewGRPCQueryRouter()
	router.SetInterfaceRegistry(cfg.InterfaceRegistry)
	types.RegisterQueryServer(router, dk)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"status": rpcserver.NewRPCFunc(func(context.Context) (*coretypes.ResultStatus, error) {
			status := &coretypes.ResultStatus{SyncInfo: coretypes.SyncInfo{LatestBlockHeight: ctx.BlockHeight(), LastCommittedBlockHeight: cms.LastCommitID().Version, LatestBlockTime: ctx.BlockTime()}}
			status.NodeInfo.Network = ctx.ChainID()
			return status, nil
		}),
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, request *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			height := int64(request.Height)
			if height == 0 {
				height = ctx.BlockHeight()
			}
			header, ok := headers[height]
			if !ok {
				return nil, fmt.Errorf("unknown committed height %d", height)
			}
			snapshot := committed[height].CacheMultiStore()
			defer snapshot.Close()
			handler := router.Route(request.Path)
			if handler == nil {
				return nil, fmt.Errorf("unknown query %s", request.Path)
			}
			// Query handlers may increment reward periods; use a private cache and
			// never write it back to the committed version used by the next CLI call.
			response, err := handler(ctx.WithMultiStore(snapshot).WithBlockHeader(header), abci.RequestQuery{Path: request.Path, Data: request.Data, Height: height})
			if err != nil {
				return nil, err
			}
			response.Height = height
			return &coretypes.ResultABCIQuery{Response: response}, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	rpc, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	status, err := rpc.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, ctx.ChainID(), status.NodeInfo.Network)
	require.Equal(t, cms.LastCommitID().Version, status.SyncInfo.LatestBlockHeight)
	clientCtx := client.Context{}.WithCodec(cfg.Codec).WithLegacyAmino(cfg.LegacyAmino).WithInterfaceRegistry(cfg.InterfaceRegistry).WithTxConfig(cfg.TxConfig).WithClient(rpc).WithChainID("repair-m673")
	s := NewIntegrationTestSuite(cfg)
	s.SetT(t)
	s.network = &network.Network{T: t, Config: cfg, Validators: []*network.Validator{{Address: address, ClientCtx: clientCtx, RPCClient: rpc}}}
	return s
}

// The legacy store cannot read past versions. Retain each block's real database
// bytes before the next BeginBlock mutates them, and reopen those committed
// fixtures with the production store loader for height-specific queries.
func repairM673CommittedStore(t *testing.T, source dbm.DB, keys map[string]*storetypes.KVStoreKey, transient *storetypes.TransientStoreKey) storetypes.CommitMultiStore {
	t.Helper()
	db := dbm.NewMemDB()
	iterator, err := source.Iterator(nil, nil)
	require.NoError(t, err)
	for ; iterator.Valid(); iterator.Next() {
		require.NoError(t, db.Set(iterator.Key(), iterator.Value()))
	}
	require.NoError(t, iterator.Error())
	require.NoError(t, iterator.Close())
	cms := store.NewCommitMultiStore(db)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	return cms
}
