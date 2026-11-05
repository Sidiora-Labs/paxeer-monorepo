// Proves: pricefeed.Keeper.SubmitPrices/Aggregate and types.GenesisState.Params -> types.PricefeedKeeper and committed SDK KV stores -> perps.Keeper.UpdatePrices/Withdrawable/RankADL.
// The producer, interface, JSON codecs, indexes and consumers are real. The
// database is closed and reopened before consumption; repeated reads and later
// producer updates prove that no cached value substitutes for the crossing.
// Repairs modules/perps/keeper/keeper.go, Keeper.GetParams: valid JSON with
// invalid parameter values must be refused at decoding, before a consumer uses it.
package types_test

import (
	"encoding/json"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	perps "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	perpsprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// proofPerpsStore owns real production keepers on a disk-backed multistore.
// No EVM or bank operation is needed by these producer and consumer methods.
type proofPerpsStore struct {
	dir       string
	db        dbm.DB
	ms        sdk.CommitMultiStore
	ctx       sdk.Context
	keys      map[string]*sdk.KVStoreKey
	transient *sdk.TransientStoreKey
	perps     *perpskeeper.Keeper
	feed      pricefeedkeeper.Keeper
	staking   stakingkeeper.Keeper
}

func newProofPerpsStore(t *testing.T) *proofPerpsStore {
	t.Helper()
	e := &proofPerpsStore{dir: t.TempDir(), keys: map[string]*sdk.KVStoreKey{}, transient: sdk.NewTransientStoreKey("proof_params_transient")}
	for _, name := range []string{types.StoreKey, pricefeedtypes.StoreKey, authtypes.StoreKey, stakingtypes.StoreKey, "proof_params"} {
		e.keys[name] = sdk.NewKVStoreKey(name)
	}
	e.open(t, tmproto.Header{Height: 1, Time: time.Unix(1_800_000_000, 0).UTC()})
	t.Cleanup(func() { require.NoError(t, e.db.Close()) })
	e.staking.SetParams(e.ctx, stakingtypes.DefaultParams())
	require.NoError(t, e.feed.InitGenesis(e.ctx, *pricefeedtypes.DefaultGenesis()))
	return e
}

func (e *proofPerpsStore) open(t *testing.T, header tmproto.Header) {
	t.Helper()
	var err error
	e.db, err = dbm.NewDB("perps-proof", dbm.GoLevelDBBackend, e.dir)
	require.NoError(t, err)
	e.ms = store.NewCommitMultiStore(e.db)
	for _, key := range e.keys {
		e.ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, e.db)
	}
	e.ms.MountStoreWithDB(e.transient, sdk.StoreTypeTransient, e.db)
	require.NoError(t, e.ms.LoadLatestVersion())
	e.ctx = sdk.NewContext(e.ms, header, false)
	registry := cdctypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	pk := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), e.keys["proof_params"], e.transient)
	ak := authkeeper.NewAccountKeeper(cdc, e.keys[authtypes.StoreKey], pk.Subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{stakingtypes.BondedPoolName: nil, stakingtypes.NotBondedPoolName: nil})
	e.staking = stakingkeeper.NewKeeper(cdc, e.keys[stakingtypes.StoreKey], ak, nil, pk.Subspace("staking"))
	e.feed = pricefeedkeeper.NewKeeper(e.keys[pricefeedtypes.StoreKey], e.staking, ak, nil)
	e.perps = perpskeeper.NewKeeper(e.keys[types.StoreKey], nil, e.feed)
}

func (e *proofPerpsStore) restart(t *testing.T) {
	t.Helper()
	header := e.ctx.BlockHeader()
	commit := e.ms.Commit(true)
	require.Positive(t, commit.Version)
	require.NoError(t, e.db.Close())
	e.open(t, header)
	require.Equal(t, commit, e.ms.LastCommitID())
}

func proofPerpsWad(value uint64) uint256.Int {
	var word uint256.Int
	word.Mul(uint256.NewInt(value), uint256.NewInt(1_000_000_000_000_000_000))
	return word
}

func (e *proofPerpsStore) registerFeeder(t *testing.T) sdk.AccAddress {
	t.Helper()
	key := ed25519.GenPrivKeyFromSecret([]byte("perps proof validator")).PubKey()
	operator := sdk.ValAddress(key.Address())
	validator, err := stakingtypes.NewValidator(operator, key, stakingtypes.Description{})
	require.NoError(t, err)
	validator.Status = stakingtypes.Bonded
	validator.Tokens = sdk.DefaultPowerReduction
	e.staking.SetValidator(e.ctx, validator)
	e.staking.SetValidatorByPowerIndex(e.ctx, validator)
	feedKey := secp256k1.GenPrivKeyFromSecret([]byte("perps proof feeder")).PubKey()
	feeder, _, err := e.feed.RegisterFeedKey(e.ctx, operator, feedKey.Bytes())
	require.NoError(t, err)
	return feeder
}

func (e *proofPerpsStore) producePrice(t *testing.T, feeder sdk.AccAddress, sequence, dollars uint64) {
	t.Helper()
	price := proofPerpsWad(dollars)
	accepted, refused, err := e.feed.SubmitPrices(e.ctx, pricefeedtypes.MsgSubmitPrices{
		Feeder: feeder.String(), Observations: []pricefeedtypes.Observation{{MarketId: 0, Sequence: sequence,
			IndexPrice: price.Dec(), MarkPrice: price.Dec(), SourceTimestampMs: uint64(e.ctx.BlockTime().UnixMilli())}},
	})
	require.NoError(t, err)
	require.Equal(t, uint32(1), accepted)
	require.Zero(t, refused)
	e.feed.Aggregate(e.ctx)
}

func TestProofPerps0x1015ProducerTransportConsumer(t *testing.T) {
	e := newProofPerpsStore(t)
	feeder := e.registerFeeder(t)
	e.producePrice(t, feeder, 1, 1000)
	require.NoError(t, perpskeeper.SeedV6120(e.ctx, *e.perps))
	// Export from the real upgrade producer, cross the public genesis JSON
	// surface, and call the module's own validator and keeper importer.
	gs := e.perps.ExportGenesis(e.ctx)
	gs.Params.AdlPositionsPerBlock = 1
	gs.Params.WithdrawFloorPpm = 200_000
	addresses := []common.Address{common.HexToAddress("0xa1"), common.HexToAddress("0xb2"), common.HexToAddress("0xc3")}
	for _, address := range addresses {
		gs.Accounts = append(gs.Accounts, types.Account{Address: address, Collateral: *uint256.NewInt(1_000_000_000), Created: true})
	}
	raw, err := json.Marshal(&gs)
	require.NoError(t, err)
	require.NoError(t, (perps.AppModuleBasic{}).ValidateGenesis(nil, nil, raw))
	var transported types.GenesisState
	require.NoError(t, json.Unmarshal(raw, &transported))
	require.NoError(t, perpskeeper.ValidateGenesis(transported))
	e.perps.InitGenesis(e.ctx, transported)
	require.Equal(t, common.HexToAddress("0x1015"), common.HexToAddress(perpsprecompile.PerpsAddress))
	initialEvents, err := e.perps.UpdatePrices(e.ctx)
	require.NoError(t, err)
	require.Len(t, initialEvents, 1)

	var ids []uint256.Int
	for i, address := range addresses {
		price := uint64(1100)
		if i == 0 {
			price = 1000
		}
		fill, err := e.perps.ApplyFill(e.ctx, perpskeeper.PositionFill{Account: address, MarketID: 0, IsLong: i == 0,
			SizeBase: proofPerpsWad(1), Price: proofPerpsWad(price), Leverage: proofPerpsWad(20)})
		require.NoError(t, err)
		ids = append(ids, fill.PositionID)
	}
	e.restart(t)
	var surface types.PricefeedKeeper = e.feed
	for read := 0; read < 2; read++ {
		price, err := surface.GetPrice(e.ctx, 0)
		require.NoError(t, err)
		require.Equal(t, sdk.NewIntWithDecimal(1000, types.USDDecimals), price.Index)
		require.True(t, price.Fresh)
		symbol, enabled, err := surface.MarketSymbol(e.ctx, 0)
		require.NoError(t, err)
		require.Equal(t, "BTC", symbol)
		require.True(t, enabled)
		market, err := e.perps.GetMarket(e.ctx, 0)
		require.NoError(t, err)
		response, err := perpskeeper.NewMsgServerImpl(e.perps).SetMarket(sdk.WrapSDKContext(e.ctx), &types.MsgSetMarket{
			Authority: e.perps.Authority(), Market: market,
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		events, err := e.perps.UpdatePrices(e.ctx)
		require.NoError(t, err)
		require.Empty(t, events)
		state, err := e.perps.GetMarketState(e.ctx, 0)
		require.NoError(t, err)
		require.Equal(t, proofPerpsWad(1000), state.IndexPrice)
		require.Equal(t, proofPerpsWad(1000), state.MarkPrice)
		require.Equal(t, price.TimestampMs, state.PriceTimestampMs)
		quote, err := e.perps.QuoteMarket(e.ctx, 0)
		require.NoError(t, err)
		require.Equal(t, price.Index.BigInt(), quote.IndexPrice)
		require.False(t, quote.PriceStale)
		withdrawable, err := e.perps.Withdrawable(e.ctx, addresses[0])
		require.NoError(t, err)
		require.Equal(t, *uint256.NewInt(800_000_000), withdrawable)
		ranked, err := e.perps.RankADL(e.ctx, ids[0], proofPerpsWad(1000), 100)
		require.NoError(t, err)
		require.Len(t, ranked, 1)
		require.Contains(t, ids[1:], ranked[0].ID)
	}

	params, err := e.perps.GetParams(e.ctx)
	require.NoError(t, err)
	params.AdlPositionsPerBlock = 2
	params.WithdrawFloorPpm = 300_000
	err = e.perps.UpdateParams(e.ctx, e.perps.ModuleAddress().String(), params)
	require.ErrorIs(t, err, types.ErrUnauthorized)
	stored, err := e.perps.GetParams(e.ctx)
	require.NoError(t, err)
	require.Equal(t, transported.Params, stored)
	require.NoError(t, e.perps.UpdateParams(e.ctx, e.perps.Authority(), params))
	e.ctx = e.ctx.WithBlockHeight(2).WithBlockTime(e.ctx.BlockTime().Add(time.Second))
	e.producePrice(t, feeder, 2, 1050)
	e.restart(t)
	for read := 0; read < 2; read++ {
		_, err := e.perps.UpdatePrices(e.ctx)
		require.NoError(t, err)
		state, err := e.perps.GetMarketState(e.ctx, 0)
		require.NoError(t, err)
		require.Equal(t, proofPerpsWad(1050), state.IndexPrice)
		require.Equal(t, proofPerpsWad(1050), state.MarkPrice)
		withdrawable, err := e.perps.Withdrawable(e.ctx, addresses[0])
		require.NoError(t, err)
		// Equity is now 1050 USD, with a 315 USD notional floor.
		require.Equal(t, *uint256.NewInt(735_000_000), withdrawable)
		ranked, err := e.perps.RankADL(e.ctx, ids[0], proofPerpsWad(1050), 100)
		require.NoError(t, err)
		require.Len(t, ranked, 2)
		require.ElementsMatch(t, ids[1:], []uint256.Int{ranked[0].ID, ranked[1].ID})
	}
	_, err = e.perps.QuoteMarket(e.ctx, 999)
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	require.ErrorIs(t, e.perps.RequireFresh(e.ctx, 999), pricefeedtypes.ErrUnknownMarket)
	unknownMarket := transported.Markets[0]
	unknownMarket.ID = 999
	_, err = perpskeeper.NewMsgServerImpl(e.perps).SetMarket(sdk.WrapSDKContext(e.ctx), &types.MsgSetMarket{
		Authority: e.perps.Authority(), Market: unknownMarket,
	})
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	_, err = e.perps.GetMarket(e.ctx, 999)
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	stale := e.ctx.WithBlockTime(e.ctx.BlockTime().Add(time.Minute))
	require.ErrorIs(t, e.perps.RequireFresh(stale, 0), types.ErrPriceStale)
	_, err = e.perps.UpdatePrices(stale)
	require.NoError(t, err)
	state, err := e.perps.GetMarketState(stale, 0)
	require.NoError(t, err)
	require.False(t, state.Fresh)
	require.Equal(t, proofPerpsWad(1050), state.MarkPrice)

	// Fault injection mutates a real producer's serialized result, preserving
	// the actual store, JSON decoder, interface and priceWord consumer.
	feedGenesis, err := e.feed.ExportGenesis(e.ctx)
	require.NoError(t, err)
	require.Len(t, feedGenesis.Aggregates, 1)
	aggregate := feedGenesis.Aggregates[0].Aggregate
	aggregate.IndexPrice = sdk.NewInt(-1)
	badPrice, err := json.Marshal(aggregate)
	require.NoError(t, err)
	e.ctx.KVStore(e.keys[pricefeedtypes.StoreKey]).Set(pricefeedtypes.AggregateKey(0), badPrice)
	before := append([]byte(nil), e.ctx.KVStore(e.keys[types.StoreKey]).Get(types.MarketStateKey(0))...)
	_, err = e.perps.UpdatePrices(e.ctx)
	require.ErrorIs(t, err, types.ErrOverflow)
	require.Equal(t, before, e.ctx.KVStore(e.keys[types.StoreKey]).Get(types.MarketStateKey(0)))
}

func TestProofPerps0x1015MalformedParamsRefusedAtConsumer(t *testing.T) {
	e := newProofPerpsStore(t)
	gs := types.DefaultGenesis()
	e.perps.InitGenesis(e.ctx, *gs)
	valid := append([]byte(nil), e.ctx.KVStore(e.keys[types.StoreKey]).Get(types.ParamsKey)...)
	for _, field := range []string{"adl_positions_per_block", "withdraw_floor_ppm"} {
		t.Run(field, func(t *testing.T) {
			var producer map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(valid, &producer))
			if field == "adl_positions_per_block" {
				producer[field] = json.RawMessage("0")
			} else {
				producer[field] = json.RawMessage("1000001")
			}
			malformed, err := json.Marshal(producer)
			require.NoError(t, err)
			badGenesis := *types.DefaultGenesis()
			require.NoError(t, json.Unmarshal(malformed, &badGenesis.Params))
			badGenesisJSON, err := json.Marshal(&badGenesis)
			require.NoError(t, err)
			require.ErrorIs(t, perpskeeper.ValidateGenesis(badGenesis), types.ErrInvalidGenesis)
			require.ErrorIs(t, (perps.AppModuleBasic{}).ValidateGenesis(nil, nil, badGenesisJSON), types.ErrInvalidGenesis)
			e.ctx.KVStore(e.keys[types.StoreKey]).Set(types.ParamsKey, malformed)
			e.restart(t)
			for read := 0; read < 2; read++ {
				_, err := e.perps.GetParams(e.ctx)
				require.ErrorIs(t, err, types.ErrInvalidParams)
				_, err = e.perps.UpdatePrices(e.ctx)
				require.ErrorIs(t, err, types.ErrInvalidParams)
				_, err = e.perps.RankADL(e.ctx, *uint256.NewInt(1), proofPerpsWad(1000), 100)
				require.ErrorIs(t, err, types.ErrInvalidParams)
			}
			require.Equal(t, malformed, e.ctx.KVStore(e.keys[types.StoreKey]).Get(types.ParamsKey))
		})
	}
	// A truncated JSON result is refused by the same decoder.
	e.ctx.KVStore(e.keys[types.StoreKey]).Set(types.ParamsKey, valid[:len(valid)-1])
	_, err := e.perps.GetParams(e.ctx)
	require.ErrorIs(t, err, types.ErrInvalidParams)
	e.ctx.KVStore(e.keys[types.StoreKey]).Set(types.ParamsKey, valid)
	params, err := e.perps.GetParams(e.ctx)
	require.NoError(t, err)
	require.Equal(t, gs.Params, params)
}
