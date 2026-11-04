package keeper_test

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	moduleimpl "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdkclient "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// The loopback node serves the real BaseApp.Query and registered module
// services over the production JSON-RPC codec. Each publish commits the
// keeper's produced store, so queries use BaseApp's normal versioned reads
// and error encoder, rather than the in-process test router's error bypass.
type repairM943Network struct {
	app    *baseapp.BaseApp
	key    sdk.StoreKey
	rpc    *rpchttp.HTTP
	client types.QueryClient
}

func newRepairM943Network(t *testing.T, e *env) *repairM943Network {
	t.Helper()
	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &sdktestutil.TestAppOpts{})
	scConfig := paxdbconfig.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	ssConfig := paxdbconfig.DefaultStateStoreConfig()
	ssConfig.AsyncWriteBuffer = 0
	app.SetCMS(storev2rootmulti.NewStore(t.TempDir(), scConfig, ssConfig, nil))
	app.MountStores(e.key)
	registry := testkeeper.EVMTestApp.InterfaceRegistry()
	app.GRPCQueryRouter().SetInterfaceRegistry(registry)
	app.MsgServiceRouter().SetInterfaceRegistry(registry)
	moduleimpl.NewAppModule(e.k).RegisterServices(module.NewConfigurator(testkeeper.EVMTestApp.AppCodec(),
		app.MsgServiceRouter(), app.GRPCQueryRouter()))
	require.NoError(t, app.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			response, err := app.Query(ctx, &abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove})
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: *response}, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	rpc, err := rpchttp.NewWithTimeout(server.URL, 5*time.Second)
	require.NoError(t, err)
	clientCtx := sdkclient.Context{}.WithClient(rpc).WithInterfaceRegistry(registry)
	return &repairM943Network{app: app, key: e.key, rpc: rpc, client: types.NewQueryClient(clientCtx)}
}

func (n *repairM943Network) publish(t *testing.T, e *env) {
	t.Helper()
	ms := n.app.CommitMultiStore().CacheMultiStore()
	header := tmproto.Header{ChainID: n.app.ChainID, Height: n.app.LastBlockHeight() + 1, Time: e.ctx.BlockTime()}
	ctx := sdk.NewContext(ms, header, false)
	store := ctx.KVStore(n.key)
	for _, entry := range storeEntries(ctx, n.key) {
		store.Delete(entry[0])
	}
	for _, entry := range storeEntries(e.ctx, e.key) {
		store.Set(entry[0], entry[1])
	}
	require.NoError(t, n.app.SetSealedStateToCommit(ctx, ms))
	_, err := n.app.Commit(context.Background())
	require.NoError(t, err)
	// BaseApp's working hash flush enqueues data before Commit's empty flush
	// advances the watermark. Wait for the actual versioned data as well.
	history := n.app.CommitMultiStore().(*storev2rootmulti.Store).GetStateStore()
	wantEntries := storeEntries(e.ctx, e.key)
	require.Eventually(t, func() bool {
		if history.GetLatestVersion() != n.app.LastBlockHeight() {
			return false
		}
		candidate, err := n.app.CreateQueryContext(n.app.LastBlockHeight(), false)
		return err == nil && reflect.DeepEqual(wantEntries, storeEntries(candidate, n.key))
	}, 5*time.Second, time.Millisecond, "historical query store has not applied the complete committed block")
	committed, err := n.app.CreateQueryContext(0, false)
	require.NoError(t, err)
	require.Equal(t, storeEntries(e.ctx, e.key), storeEntries(committed, n.key))
	for _, entry := range storeEntries(e.ctx, e.key) {
		require.Equal(t, entry[1], committed.KVStore(n.key).Get(entry[0]), "committed point read for key %x", entry[0])
	}
}

func TestRepairM_943_f06_NFTMarketProjection(t *testing.T) {
	e := newEnv(t)
	a, b := e.account(10_000_000), e.account(10_000_000)
	ownerA, ownerB := e.evmAddress(a), e.evmAddress(b)
	market := e.create(a, types.FeeStrategyClaim)
	other := e.create(a, types.FeeStrategyLpRewards)
	e.at(mature)
	fees := sdk.NewIntFromBigInt(e.swap(a, market.Denom, true, 1_000_000).PoolCut)
	require.True(t, fees.IsPositive())
	collection := common.HexToAddress(market.FeeRightCollection)
	id := new(big.Int).SetUint64(market.FeeRightID)
	nftABI := proofModuleABI(t, types.RoleFeeRights)
	network := newRepairM943Network(t, e)
	check := func(owner common.Address, strategy types.FeeStrategy) {
		t.Helper()
		stored := e.market(market.Denom)
		require.Equal(t, strategy, stored.FeeStrategy)
		require.Equal(t, testkeeper.EVMTestApp.EvmKeeper.GetPaxAddressOrDefault(e.ctx, owner).String(), stored.FeeRightsHolder)
		network.publish(t, e)
		response, err := network.client.Market(context.Background(), &types.QueryMarketRequest{Denom: market.Denom})
		require.NoError(t, err)
		right, err := network.client.FeeRight(context.Background(), &types.QueryFeeRightRequest{Collection: collection.Hex(), Id: market.FeeRightID})
		require.NoError(t, err)
		require.True(t, right.Found)
		require.Equal(t, owner.Hex(), right.FeeRight.Owner)
		require.Equal(t, uint32(strategy), right.FeeRight.Strategy)
		require.Equal(t, testkeeper.EVMTestApp.EvmKeeper.GetPaxAddressOrDefault(e.ctx, owner).String(), response.Market.FeeRightsHolder)
		require.Equal(t, right.FeeRight.Strategy, response.Market.FeeStrategy)
		byAddress, err := network.client.MarketByAddress(context.Background(), &types.QueryMarketByAddressRequest{Address: market.PoolAddress})
		require.NoError(t, err)
		require.Equal(t, response.Market, byAddress.Market)
		markets, err := network.client.Markets(context.Background(), &types.QueryMarketsRequest{Limit: 10})
		require.NoError(t, err)
		require.Len(t, markets.Markets, 2)
		require.Equal(t, response.Market, markets.Markets[0])
		require.Equal(t, types.MarketInfoOf(other), markets.Markets[1])
		stored, _, err = e.k.RequireMarketFeeRight(e.ctx, owner, market.Denom, strategy)
		require.NoError(t, err)
		require.Equal(t, stored.FeeRightsHolder, response.Market.FeeRightsHolder)
		evm, _ := proofModuleEVM(t, e.k, e.ctx)
		require.Equal(t, owner, proofModuleView(t, evm, owner, collection, nftABI, "ownerOf", id))
		require.Equal(t, uint8(strategy), proofModuleView(t, evm, owner, collection, nftABI, "getFeeStrategy", id))
	}
	mutate := func(caller common.Address, name string, args ...interface{}) {
		t.Helper()
		evm, db := proofModuleEVM(t, e.k, e.ctx)
		input, err := nftABI.Pack(name, args...)
		require.NoError(t, err)
		_, _, err = evm.Call(caller, collection, input, 30_000_000, uint256.NewInt(0))
		require.NoError(t, err)
		e.ctx = db.Ctx()
	}
	check(ownerA, types.FeeStrategyClaim)
	mutate(ownerA, "transferFrom", ownerA, ownerB, id)
	check(ownerB, types.FeeStrategyClaim)
	_, err := e.k.ClaimFees(e.ctx, a, market.Denom, a)
	require.ErrorIs(t, err, types.ErrNotFeeRightsHolder)
	mutate(ownerB, "setFeeStrategy", id, uint8(types.FeeStrategyBurn))
	check(ownerB, types.FeeStrategyBurn)
	_, err = e.k.ClaimFees(e.ctx, b, market.Denom, b)
	require.ErrorIs(t, err, types.ErrWrongStrategy)
	repair34Restart(t, e)
	check(ownerB, types.FeeStrategyBurn)
	mutate(ownerB, "setFeeStrategy", id, uint8(types.FeeStrategyClaim))
	check(ownerB, types.FeeStrategyClaim)
	before := e.usdlOf(b)
	claimed, err := e.k.ClaimFees(e.ctx, b, market.Denom, b)
	require.NoError(t, err)
	require.Equal(t, fees, claimed)
	require.Equal(t, before.Add(fees), e.usdlOf(b))
	mutate(ownerB, "transferFrom", ownerB, ownerA, id)
	check(ownerA, types.FeeStrategyClaim)
	repair34Restart(t, e)
	check(ownerA, types.FeeStrategyClaim)
}

func TestRepairM_943_f07_AccountingCountsNetwork(t *testing.T) {
	e := newEnv(t)
	network := newRepairM943Network(t, e)
	check := func(pools, epochs, claims uint64, liabilities sdk.Int) {
		t.Helper()
		storedLiabilities, err := e.k.TotalLiabilities(e.ctx)
		require.NoError(t, err)
		require.Equal(t, liabilities, storedLiabilities, "producer liabilities before network publication")
		network.publish(t, e)
		response, err := network.client.FeeAccounting(context.Background(), &types.QueryFeeAccountingRequest{})
		require.NoError(t, err)
		require.Equal(t, types.FeeAccountingInfo{PoolCount: pools, EpochCount: epochs, ClaimCount: claims,
			PendingProtocolFees: sdk.ZeroInt(), TotalLiabilities: liabilities}, response.Accounting)
		persisted := e.k.ExportGenesis(e.ctx).FeeAccounting
		require.Len(t, persisted.Pools, int(pools))
		require.Len(t, persisted.Epochs, int(epochs))
		require.Len(t, persisted.Claims, int(claims))
		require.Equal(t, liabilities, persisted.TotalLiabilities)
	}
	check(0, 0, 0, sdk.ZeroInt())
	creator, holder, other := e.account(10_000_000), e.account(10_000_000), e.account(10_000_000)
	market := e.create(creator, types.FeeStrategyAirdrop)
	e.create(creator, types.FeeStrategyClaim)
	pool := common.HexToAddress(market.PoolAddress)
	roles := e.k.Generation2()
	buy := func(trader sdk.AccAddress, amount int64) keeper.SwapResult {
		t.Helper()
		before, err := e.k.TotalLiabilities(e.ctx)
		require.NoError(t, err)
		want, err := e.k.QuoteBuy(e.ctx, market.Denom, big.NewInt(amount))
		require.NoError(t, err)
		contract := proofModuleABI(t, types.RoleRouter)
		input, err := contract.Pack("buy", pool, big.NewInt(amount), want.AmountOut, new(big.Int).SetUint64(farDeadline))
		require.NoError(t, err)
		evm, db := proofModuleEVM(t, e.k, e.ctx)
		raw, _, err := evm.Call(e.evmAddress(trader), roles.Router, input, 30_000_000, uint256.NewInt(0))
		require.NoError(t, err, "Router.buy: %x", raw)
		e.ctx = db.Ctx()
		values, err := contract.Unpack("buy", raw)
		require.NoError(t, err)
		require.Equal(t, want.AmountOut, values[0])
		after, err := e.k.TotalLiabilities(e.ctx)
		require.NoError(t, err)
		require.Equal(t, before.Add(sdk.NewIntFromBigInt(want.PoolCut)), after, "Router.buy must persist its pool-cut liability")
		return want
	}
	// Reinstalling a surface updates one record, rather than adding a pool.
	surface, found, err := e.k.GetSurface(e.ctx, pool)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, e.k.SetSurface(e.ctx, pool, surface))
	check(2, 0, 0, sdk.ZeroInt())
	e.ctx = e.ctx.WithBlockHeight(2)
	e.at(mature)
	first := buy(holder, 2_000_000)
	second := buy(other, 1_000_000)
	fees := sdk.NewIntFromBigInt(new(big.Int).Add(first.PoolCut, second.PoolCut))
	require.True(t, fees.IsPositive())
	check(2, 0, 0, fees)
	_, err = e.k.GovernanceGrant(e.ctx, e.k.Authority(), roles.FeeAccumulator, keeper.FeesRouterRole, roles.FeesRouter)
	require.NoError(t, err)
	e.ctx = e.ctx.WithBlockHeight(3)
	triggered, err := e.k.TriggerAirdrop(e.ctx, e.k.ModuleCaller(), roles.FeesRouter, pool)
	require.NoError(t, err)
	require.Equal(t, fees, triggered)
	check(2, 1, 0, fees)
	share := fees.Mul(sdk.NewIntFromBigInt(first.AmountOut)).Quo(sdk.NewIntFromBigInt(new(big.Int).Add(first.AmountOut, second.AmountOut)))
	before := e.usdlOf(holder)
	e.ctx, triggered, err = e.k.ClaimAirdropEpoch(e.ctx, e.k.ModuleCaller(), pool, e.evmAddress(holder), 1)
	require.NoError(t, err)
	require.Equal(t, share, triggered)
	require.Equal(t, before.Add(share), e.usdlOf(holder))
	check(2, 1, 1, fees.Sub(share))
	// Updating the remaining amount of epoch 1 does not create epoch 2.
	record, found, err := e.k.GetPoolAirdropEpoch(e.ctx, pool, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, e.k.SetPoolAirdropEpoch(e.ctx, record))
	check(2, 1, 1, fees.Sub(share))
	repair34Restart(t, e)
	check(2, 1, 1, fees.Sub(share))
	before = e.usdlOf(other)
	e.ctx, triggered, err = e.k.ClaimAirdropEpoch(e.ctx, e.k.ModuleCaller(), pool, e.evmAddress(other), 1)
	require.NoError(t, err)
	require.Equal(t, fees.Sub(share), triggered)
	require.Equal(t, before.Add(triggered), e.usdlOf(other))
	check(2, 1, 2, sdk.ZeroInt())
	_, _, err = e.k.ClaimAirdropEpoch(e.ctx, e.k.ModuleCaller(), pool, e.evmAddress(holder), 1)
	require.ErrorIs(t, err, types.ErrAlreadyClaimed)
	check(2, 1, 2, sdk.ZeroInt())
	e.ctx = e.ctx.WithBlockHeight(4)
	third := buy(holder, 1_000_000)
	newFees := sdk.NewIntFromBigInt(third.PoolCut)
	e.ctx = e.ctx.WithBlockHeight(5)
	triggered, err = e.k.TriggerAirdrop(e.ctx, e.k.ModuleCaller(), roles.FeesRouter, pool)
	require.NoError(t, err)
	require.Equal(t, newFees, triggered)
	check(2, 2, 2, newFees)
	repair34Restart(t, e)
	check(2, 2, 2, newFees)
}

func TestRepairM_943_f22_RefusalNetwork(t *testing.T) {
	// Retain the nil request, exact returned-error and store-preservation proof.
	TestQueryFeeAccountingRefusals(t)
	for _, raw := range [][]byte{[]byte("invalid"), []byte("-1"), []byte(new(big.Int).Lsh(big.NewInt(1), 256).String())} {
		t.Run(string(raw), func(t *testing.T) {
			e := newEnv(t)
			e.create(e.account(10_000_000), types.FeeStrategyClaim)
			store := e.ctx.KVStore(e.key)
			store.Set([]byte{0x49}, raw)
			before := storeEntries(e.ctx, e.key)
			response, produced := keeper.NewQuerier(e.k).FeeAccounting(sdk.WrapSDKContext(e.ctx), &types.QueryFeeAccountingRequest{})
			require.Nil(t, response)
			require.ErrorIs(t, produced, types.ErrOverflow)
			space, code, log := sdkerrors.ABCIInfo(produced, false)
			require.Equal(t, types.ModuleName, space)
			require.Equal(t, types.ErrOverflow.ABCICode(), code)
			network := newRepairM943Network(t, e)
			network.publish(t, e)
			request, err := (&types.QueryFeeAccountingRequest{}).Marshal()
			require.NoError(t, err)
			wire, err := network.rpc.ABCIQuery(context.Background(), "/paxprotocol.paxchain.launchpad.Query/FeeAccounting", request)
			require.NoError(t, err)
			require.Equal(t, space, wire.Response.Codespace)
			require.Equal(t, code, wire.Response.Code)
			require.Equal(t, log, wire.Response.Log)
			require.Empty(t, wire.Response.Value)
			require.ErrorIs(t, sdkerrors.ABCIError(wire.Response.Codespace, wire.Response.Code, wire.Response.Log), types.ErrOverflow)
			committed, err := network.app.CreateQueryContext(0, false)
			require.NoError(t, err)
			require.Equal(t, before, storeEntries(committed, network.key))
			require.Equal(t, before, storeEntries(e.ctx, e.key))
		})
	}
}

func TestRepairM_943_f08_ViewsUnderActualPoolLock(t *testing.T) {
	TestViewsAnswerUnderLock(t)
}
