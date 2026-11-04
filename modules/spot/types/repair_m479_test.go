package types_test

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	feedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	feedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	spotkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
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
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const repairM479Base = int64(1_800_000_000)

var repairM479Token = common.HexToAddress("0xa479")

type repairM479Store struct {
	dir       string
	db        dbm.DB
	ms        sdk.CommitMultiStore
	ctx       sdk.Context
	keys      map[string]*sdk.KVStoreKey
	transient *sdk.TransientStoreKey
	k         *spotkeeper.Keeper
	feed      feedkeeper.Keeper
	staking   stakingkeeper.Keeper
}

func newRepairM479Store(t *testing.T) *repairM479Store {
	t.Helper()
	e := &repairM479Store{dir: t.TempDir(), keys: map[string]*sdk.KVStoreKey{},
		transient: sdk.NewTransientStoreKey("repair_m479_transient")}
	for _, name := range []string{types.StoreKey, feedtypes.StoreKey, authtypes.StoreKey, stakingtypes.StoreKey,
		evmtypes.StoreKey, "repair_m479_params"} {
		e.keys[name] = sdk.NewKVStoreKey(name)
	}
	e.open(t, tmproto.Header{Height: 1, Time: time.Unix(repairM479Base, 0).UTC()})
	t.Cleanup(func() {
		if e.db != nil {
			require.NoError(t, e.db.Close())
		}
	})
	return e
}

func (e *repairM479Store) open(t *testing.T, header tmproto.Header) {
	t.Helper()
	var err error
	e.db, err = dbm.NewGoLevelDB("repair_m479", e.dir)
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
	pk := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), e.keys["repair_m479_params"], e.transient)
	ak := authkeeper.NewAccountKeeper(cdc, e.keys[authtypes.StoreKey], pk.Subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: nil, stakingtypes.BondedPoolName: nil, stakingtypes.NotBondedPoolName: nil})
	e.staking = stakingkeeper.NewKeeper(cdc, e.keys[stakingtypes.StoreKey], ak, nil, pk.Subspace("staking"))
	evm := evmkeeper.NewKeeper(e.keys[evmtypes.StoreKey], e.transient, pk.Subspace("evm"), nil, nil, &ak, &e.staking,
		transferkeeper.Keeper{}, nil, nil, nil)
	e.feed = feedkeeper.NewKeeper(e.keys[feedtypes.StoreKey], e.staking, ak, evm)
	e.k = spotkeeper.NewKeeper(e.keys[types.StoreKey], nil, evm, e.feed)
}

func (e *repairM479Store) stop(t *testing.T) {
	t.Helper()
	require.Positive(t, e.ms.Commit(true).Version)
	require.NoError(t, e.db.Close())
	e.db, e.ms, e.k = nil, nil, nil
	e.ctx = sdk.Context{}
}

func repairM479Produce(t *testing.T, count int) *repairM479Store {
	t.Helper()
	e := newRepairM479Store(t)
	gs := types.DefaultGenesis()
	gs.Params.Tokens = types.OpeningTokens(repairM479Token, common.HexToAddress("0xb479"))
	e.k.InitGenesis(e.ctx, *gs)
	e.staking.SetParams(e.ctx, stakingtypes.DefaultParams())
	require.NoError(t, e.feed.InitGenesis(e.ctx, *feedtypes.DefaultGenesis()))
	pub := ed25519.GenPrivKeyFromSecret([]byte("repair m479 validator")).PubKey()
	operator := sdk.ValAddress(pub.Address())
	validator, err := stakingtypes.NewValidator(operator, pub, stakingtypes.Description{})
	require.NoError(t, err)
	validator.Status, validator.Tokens = stakingtypes.Bonded, sdk.DefaultPowerReduction
	e.staking.SetValidator(e.ctx, validator)
	e.staking.SetValidatorByPowerIndex(e.ctx, validator)
	feeder, _, err := e.feed.RegisterFeedKey(e.ctx, operator,
		secp256k1.GenPrivKeyFromSecret([]byte("repair m479 feeder")).PubKey().Bytes())
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		e.ctx = e.ctx.WithBlockHeight(int64(i + 2)).WithBlockTime(time.Unix(repairM479Base+int64(i/2)*2, 0).UTC())
		price := sdk.NewIntWithDecimal(int64(1000+i), 18)
		accepted, refused, err := e.feed.SubmitPrices(e.ctx, feedtypes.MsgSubmitPrices{Feeder: feeder.String(),
			Observations: []feedtypes.Observation{{MarketId: types.PAXMarketID, Sequence: uint64(i + 1),
				IndexPrice: price.String(), MarkPrice: price.String(), SourceTimestampMs: uint64(e.ctx.BlockTime().UnixMilli())}}})
		require.NoError(t, err)
		require.Equal(t, uint32(1), accepted)
		require.Zero(t, refused)
		e.feed.Aggregate(e.ctx)
		livePrice, err := e.k.IndexPrice(e.ctx, repairM479Token)
		require.NoError(t, err)
		require.Equal(t, price, livePrice)
		require.NoError(t, e.k.RecordSample(e.ctx, repairM479Token))
	}
	return e
}

func repairM479Transport(t *testing.T, gs *types.GenesisState) types.GenesisState {
	t.Helper()
	bz, err := json.Marshal(gs)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, json.Unmarshal(bz, &decoded))
	return decoded
}

func repairM479Snapshot(t *testing.T, e *repairM479Store) map[string]string {
	t.Helper()
	values := map[string]string{}
	it := e.ctx.KVStore(e.keys[types.StoreKey]).Iterator(nil, nil)
	defer func() { require.NoError(t, it.Close()) }()
	for ; it.Valid(); it.Next() {
		values[string(it.Key())] = string(it.Value())
	}
	return values
}

func repairM479Reject(t *testing.T, gs types.GenesisState) {
	t.Helper()
	err := gs.Validate()
	require.ErrorIs(t, err, types.ErrInvalidGenesis)
	target := newRepairM479Store(t)
	before := repairM479Snapshot(t, target)
	require.PanicsWithError(t, "spot: invalid genesis: "+err.Error(), func() { target.k.InitGenesis(target.ctx, gs) })
	require.Equal(t, before, repairM479Snapshot(t, target), "invalid history must not reach the store")
}

func TestRepairM_479_RejectMalformedRings(t *testing.T) {
	source := repairM479Produce(t, 2)
	gs := source.k.ExportGenesis(source.ctx)
	require.NoError(t, gs.Validate())
	cases := []struct {
		name   string
		mutate func(*types.GenesisState)
	}{
		{"sparse", func(g *types.GenesisState) { g.TwapSamples[0].Slot, g.TwapSamples[1].Slot = 7, 8 }},
		{"wrong_head", func(g *types.GenesisState) { g.TwapRings[0].Head = 3 }},
		{"decreasing_time", func(g *types.GenesisState) { g.TwapSamples[0].Time++ }},
		{"zero_price", func(g *types.GenesisState) { g.TwapSamples[0].Index = sdk.ZeroInt() }},
		{"negative_price", func(g *types.GenesisState) { g.TwapSamples[0].Index = sdk.NewInt(-1) }},
		{"duplicate_slot", func(g *types.GenesisState) { g.TwapSamples[1].Slot = g.TwapSamples[0].Slot }},
		{"missing_sample", func(g *types.GenesisState) { g.TwapSamples = g.TwapSamples[:1] }},
		{"extra_sample", func(g *types.GenesisState) {
			sample := g.TwapSamples[0]
			sample.Slot = 2
			g.TwapSamples = append(g.TwapSamples, sample)
		}},
		{"wrong_token", func(g *types.GenesisState) { g.TwapSamples[0].Token = common.HexToAddress("0xb479") }},
		{"wrong_ring_token", func(g *types.GenesisState) { g.TwapRings[0].Token = common.HexToAddress("0xb479") }},
		{"slot_out_of_bounds", func(g *types.GenesisState) { g.TwapSamples[0].Slot = types.TwapHistoryDepth }},
		{"head_out_of_bounds", func(g *types.GenesisState) { g.TwapRings[0].Head = types.TwapHistoryDepth }},
		{"count_out_of_bounds", func(g *types.GenesisState) { g.TwapRings[0].Count = types.TwapHistoryDepth + 1 }},
		{"duplicate_ring", func(g *types.GenesisState) { g.TwapRings = append(g.TwapRings, g.TwapRings[0]) }},
		{"empty_ring_with_samples", func(g *types.GenesisState) { g.TwapRings[0].Count = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := repairM479Transport(t, gs)
			tc.mutate(&candidate)
			repairM479Reject(t, repairM479Transport(t, &candidate))
		})
	}
	t.Run("nil_price", func(t *testing.T) {
		candidate := repairM479Transport(t, gs)
		candidate.TwapSamples[0].Index = sdk.Int{}
		repairM479Reject(t, candidate)
	})
}

func repairM479Expected(total int, now, period int64) sdk.Int {
	sum := new(big.Int)
	for i := max(0, total-types.TwapHistoryDepth); i < total; i++ {
		start := max(repairM479Base+int64(i/2)*2, now-period)
		end := now
		if i+1 < total {
			end = min(now, repairM479Base+int64((i+1)/2)*2)
		}
		if end > start {
			sum.Add(sum, new(big.Int).Mul(sdk.NewIntWithDecimal(int64(1000+i), 18).BigInt(), big.NewInt(end-start)))
		}
	}
	return sdk.NewIntFromBigInt(sum.Quo(sum, big.NewInt(period)))
}

func TestRepairM_479_WrappedRingRestartRecovery(t *testing.T) {
	const total = types.TwapHistoryDepth + 3
	source := repairM479Produce(t, total)
	source.ctx = source.ctx.WithBlockTime(source.ctx.BlockTime().Add(3 * time.Second))
	header := source.ctx.BlockHeader()
	gs := source.k.ExportGenesis(source.ctx)
	require.Equal(t, []types.TwapRing{{Token: repairM479Token, Head: 3, Count: types.TwapHistoryDepth}}, gs.TwapRings)
	require.Len(t, gs.TwapSamples, types.TwapHistoryDepth)
	require.Greater(t, gs.TwapSamples[0].Time, gs.TwapSamples[len(gs.TwapSamples)-1].Time,
		"physical slot order differs from chronological order after wrapping")
	require.NoError(t, gs.Validate())
	t.Run("decreasing_time_across_wrap", func(t *testing.T) {
		candidate := repairM479Transport(t, gs)
		candidate.TwapSamples[0].Time = candidate.TwapSamples[types.TwapHistoryDepth-1].Time - 1
		repairM479Reject(t, candidate)
	})
	periods := []int64{1, 3, 4, 17, 500, 1003}
	for _, period := range periods {
		got, err := source.k.TWAP(source.ctx, repairM479Token, uint64(period))
		require.NoError(t, err)
		require.Equal(t, repairM479Expected(total, header.Time.Unix(), period), got, "live period %d", period)
	}
	bz, err := json.Marshal(gs)
	require.NoError(t, err)
	source.stop(t)
	source.open(t, header)
	require.Equal(t, gs, source.k.ExportGenesis(source.ctx), "committed history survives closing and reopening")
	source.stop(t)

	target := newRepairM479Store(t)
	target.ctx = target.ctx.WithBlockHeader(header)
	require.Empty(t, repairM479Snapshot(t, target))
	var recovered types.GenesisState
	require.NoError(t, json.Unmarshal(bz, &recovered))
	require.NoError(t, recovered.Validate())
	target.k.InitGenesis(target.ctx, recovered)
	require.Equal(t, gs, target.k.ExportGenesis(target.ctx))
	target.stop(t)
	target.open(t, header)
	require.Equal(t, gs, target.k.ExportGenesis(target.ctx))
	for _, period := range periods {
		got, err := target.k.TWAP(target.ctx, repairM479Token, uint64(period))
		require.NoError(t, err)
		require.Equal(t, repairM479Expected(total, header.Time.Unix(), period), got, "recovered period %d", period)
	}
	_, err = target.k.TWAP(target.ctx, repairM479Token, 1004)
	require.ErrorIs(t, err, types.ErrTwapWindowInvalid, "overwritten history remains unavailable")
	require.NoError(t, target.k.RecordSample(target.ctx, common.HexToAddress(types.USDLAddress)))
	require.NoError(t, target.k.ExportGenesis(target.ctx).Validate(), "recovered keeper still produces valid history")
}

func TestRepairM_479_EmptyAndPartialRingRecovery(t *testing.T) {
	for _, count := range []int{0, 1, 2, 5} {
		t.Run(big.NewInt(int64(count)).String(), func(t *testing.T) {
			source := repairM479Produce(t, count)
			gs := source.k.ExportGenesis(source.ctx)
			require.NoError(t, gs.Validate())
			transported := repairM479Transport(t, gs)
			header := source.ctx.BlockHeader()
			source.stop(t)
			target := newRepairM479Store(t)
			target.ctx = target.ctx.WithBlockHeader(header)
			target.k.InitGenesis(target.ctx, transported)
			target.stop(t)
			target.open(t, header)
			require.Equal(t, gs, target.k.ExportGenesis(target.ctx))
			if count > 0 {
				target.ctx = target.ctx.WithBlockTime(header.Time.Add(time.Second))
				got, err := target.k.TWAP(target.ctx, repairM479Token, 1)
				require.NoError(t, err)
				require.Equal(t, sdk.NewIntWithDecimal(int64(999+count), 18), got)
			} else {
				_, err := target.k.TWAP(target.ctx, repairM479Token, 1)
				require.ErrorIs(t, err, types.ErrTwapWindowInvalid)
			}
		})
	}
}
