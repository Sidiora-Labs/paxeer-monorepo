package keeper

import (
	"math/big"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// repair14Reopen commits the actual IAVL store, releases the keeper and opens
// a new multistore and keeper against the same committed database.
func repair14Reopen(t *testing.T, k *Keeper, ctx sdk.Context) (*Keeper, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(k.storeKey, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())
	persisted := sdk.NewContext(ms, tmproto.Header{Height: ctx.BlockHeight(), Time: ctx.BlockTime()}, false)
	it := ctx.KVStore(k.storeKey).Iterator(nil, nil)
	for ; it.Valid(); it.Next() {
		persisted.KVStore(k.storeKey).Set(it.Key(), it.Value())
	}
	require.NoError(t, it.Close())
	ms.Commit(true)
	reopened := store.NewCommitMultiStore(db)
	reopened.MountStoreWithDB(k.storeKey, sdk.StoreTypeIAVL, db)
	require.NoError(t, reopened.LoadLatestVersion())
	return &Keeper{storeKey: k.storeKey, authority: k.authority, evm: k.evm, pricefeed: k.pricefeed}, sdk.NewContext(reopened, tmproto.Header{Height: ctx.BlockHeight(), Time: ctx.BlockTime()}, false)
}

func TestRepair_14_adl_uses_current_mark(t *testing.T) {
	k, ctx := adlTestKeeper(t)
	ctx = ctx.WithBlockHeight(1)
	state, err := k.GetMarketState(ctx, 0)
	require.NoError(t, err)
	state.MarkPrice = adlUSD(60_000)
	k.SetMarketState(ctx, state)
	deficit := adlPositionFixture(99, true, adlUSD(1), adlUSD(70_000), uint256.Int{})
	deficit.Account = k.InsuranceAccountAddress()
	adlSeedPosition(t, k, ctx, deficit, uint256.Int{}, adlUSD(2_000))
	// The deficit position was settled at the previous committed mark.
	deficit, err = k.GetPosition(ctx, deficit.ID)
	require.NoError(t, err)
	deficit.ReferencePrice = adlUSD(60_000)
	deficit.CostBasisUsd = adlUSD(60_000)
	k.store(ctx).Set(types.PositionKey(deficit.ID), deficit.Marshal())
	counterparty := adlPositionFixture(1, false, adlUSD(1), adlUSD(66_000), adlUSD(5))
	adlSeedPosition(t, k, ctx, counterparty, *uint256.NewInt(12_000_000_000), uint256.Int{})
	ctx = ctx.WithBlockHeight(2)
	state, err = k.GetMarketState(ctx, 0)
	require.NoError(t, err)
	require.NoError(t, k.snapshotCommittedMark(ctx, state))
	state.MarkPrice = adlUSD(65_000)
	k.SetMarketState(ctx, state)
	k, ctx = repair14Reopen(t, k, ctx)
	ids, marks, err := k.insuranceAdlInputs(ctx, k.InsuranceAccountAddress())
	require.NoError(t, err)
	require.Equal(t, adlUSD(60_000), marks[0])
	events, err := k.ExecuteADL(ctx, ids, marks)
	require.NoError(t, err)
	matches := adlEvents(events, types.EventADLMatched)
	require.Len(t, matches, 1)
	require.Equal(t, adlBig(60_000), matches[0].Args[4])
	payer, err := k.GetAccount(ctx, counterparty.Account)
	require.NoError(t, err)
	require.Equal(t, *uint256.NewInt(16_000_000_000), payer.Collateral)
}

func TestRepair_14_settlement_leaves_adl_rank(t *testing.T) {
	k, ctx := adlTestKeeper(t)
	state, err := k.GetMarketState(ctx, 0)
	require.NoError(t, err)
	state.MarkPrice = adlUSD(60_000)
	k.SetMarketState(ctx, state)
	first := adlPositionFixture(1, false, adlUSD(1), adlUSD(66_000), adlUSD(99))
	second := adlPositionFixture(2, false, adlUSD(1), adlUSD(66_000), adlUSD(98))
	deficit := adlPositionFixture(99, true, adlUSD(2), adlUSD(70_000), uint256.Int{})
	adlSeedPosition(t, k, ctx, first, *uint256.NewInt(10_000_000_000), uint256.Int{})
	adlSeedPosition(t, k, ctx, second, *uint256.NewInt(20_000_000_000), uint256.Int{})
	adlSeedPosition(t, k, ctx, deficit, uint256.Int{}, adlUSD(2_000))
	for _, id := range []uint256.Int{first.ID, second.ID} {
		p, loadErr := k.GetPosition(ctx, id)
		require.NoError(t, loadErr)
		p.Leverage = adlUSD(10)
		k.store(ctx).Set(types.PositionKey(id), p.Marshal())
	}
	_, err = k.Settle(ctx, first.Account)
	require.NoError(t, err)
	_, err = k.Settle(ctx, second.Account)
	require.NoError(t, err)
	p1, err := k.GetPosition(ctx, first.ID)
	require.NoError(t, err)
	a1, err := k.Equity(ctx, first.Account)
	require.NoError(t, err)
	require.Equal(t, positionRank(p1, a1.BigInt()), p1.RankScore)
	ranked, err := k.RankADL(ctx, deficit.ID, adlUSD(60_000), 2)
	require.NoError(t, err)
	require.Equal(t, first.ID, ranked[0].ID)
	_, err = k.Deposit(ctx, first.Account, *uint256.NewInt(20_000_000_000))
	require.NoError(t, err)
	ranked, err = k.RankADL(ctx, deficit.ID, adlUSD(60_000), 2)
	require.NoError(t, err)
	require.Equal(t, second.ID, ranked[0].ID)
	_, err = k.Withdraw(ctx, first.Account, *uint256.NewInt(20_000_000_000))
	require.NoError(t, err)
	ranked, err = k.RankADL(ctx, deficit.ID, adlUSD(60_000), 2)
	require.NoError(t, err)
	require.Equal(t, first.ID, ranked[0].ID)
	k, ctx = repair14Reopen(t, k, ctx)
	ranked, err = k.RankADL(ctx, deficit.ID, adlUSD(60_000), 2)
	require.NoError(t, err)
	require.Equal(t, first.ID, ranked[0].ID)
	events, err := k.ExecuteADL(ctx, []uint256.Int{deficit.ID}, map[uint64]uint256.Int{0: adlUSD(60_000)})
	require.NoError(t, err)
	matches := adlEvents(events, types.EventADLMatched)
	require.NotEmpty(t, matches)
	require.Equal(t, first.ID.ToBig(), matches[0].Args[1])
	require.Equal(t, new(big.Int).SetUint64(0), matches[0].Args[0])
}

func TestRepair_14_adl_index_visit_bound(t *testing.T) {
	k, ctx := adlTestKeeper(t)
	params := types.DefaultParams()
	params.AdlPositionsPerBlock = 2
	require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
	deficit := adlPositionFixture(99, true, adlUSD(1), adlUSD(70_000), uint256.Int{})
	adlSeedPosition(t, k, ctx, deficit, uint256.Int{}, adlUSD(2_000))
	for id := uint64(1); id <= 7; id++ {
		entry := adlUSD(59_000)
		if id == 7 {
			entry = adlUSD(66_000)
		}
		p := adlPositionFixture(id, false, adlUSD(1), entry, adlUSD(8-id))
		adlSeedPosition(t, k, ctx, p, *uint256.NewInt(20_000_000_000), uint256.Int{})
	}
	// Balance the seven shorts with six additional long units.
	balancing := adlPositionFixture(98, true, adlUSD(6), adlUSD(60_000), uint256.Int{})
	adlSeedPosition(t, k, ctx, balancing, *uint256.NewInt(400_000_000_000), uint256.Int{})
	marks := map[uint64]uint256.Int{0: adlUSD(60_000)}
	for pass := uint64(0); pass < 3; pass++ {
		events, err := k.ExecuteADL(ctx, []uint256.Int{deficit.ID}, marks)
		require.NoError(t, err)
		require.Empty(t, adlEvents(events, types.EventADLMatched))
		budget, err := k.loadADLBudget(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(2), budget.visits)
		cursor := append([]byte(nil), k.store(ctx).Get(adlCursorKey(deficit.ID))...)
		require.NotEmpty(t, cursor)
		// A second invocation in the same block cannot renew the allowance.
		events, err = k.ExecuteADL(ctx, []uint256.Int{deficit.ID}, marks)
		require.NoError(t, err)
		require.Empty(t, events)
		require.Equal(t, cursor, k.store(ctx).Get(adlCursorKey(deficit.ID)))
		k, ctx = repair14Reopen(t, k, ctx)
		require.Equal(t, cursor, k.store(ctx).Get(adlCursorKey(deficit.ID)))
		ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	}
	events, err := k.ExecuteADL(ctx, []uint256.Int{deficit.ID}, marks)
	require.NoError(t, err)
	matches := adlEvents(events, types.EventADLMatched)
	require.Len(t, matches, 1)
	require.Equal(t, uint256.NewInt(7).ToBig(), matches[0].Args[1])
}
