package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/stretchr/testify/require"
)

func TestValidateGenesisRefusesMismatchedIndexes(t *testing.T) {
	orphan := *types.DefaultGenesis()
	orphan.BookEntries = []types.BookEntry{{
		MarketID: 0, Side: types.SideBuy, Price: genesisE18(59_000), Sequence: 1, OrderID: genesisWord(1),
	}}
	mismatched := scenarioGenesis()
	mismatched.BookEntries = []types.BookEntry{
		{MarketID: 0, Side: types.SideBuy, Price: genesisE18(59_001), Sequence: 1, OrderID: genesisWord(1)},
		{MarketID: 0, Side: types.SideSell, Price: genesisE18(61_000), Sequence: 2, OrderID: genesisWord(2)},
		{MarketID: 3, Side: types.SideBuy, Price: genesisE18(2_990), Sequence: 3, OrderID: genesisWord(3)},
		{MarketID: 7, Side: types.SideSell, Price: genesisE18(150), Sequence: 8, OrderID: genesisWord(8)},
	}
	invalidRecords := scenarioGenesis()
	invalidRecords.Counters.NextOrderID = genesisWord(1)
	invalidTrigger := scenarioGenesis()
	invalidTrigger.Orders[3].Type = types.OrderTypeLimit

	for _, tc := range []struct {
		name    string
		gs      types.GenesisState
		message string
	}{
		{"orphan book entry", orphan, "1 book entries where the records give 0"},
		{"mismatched book key", mismatched, "disagrees with the records"},
		{"invalid records", invalidRecords, "next order id"},
		{"invalid trigger", invalidTrigger, "has no trigger"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs := tc.gs
			gs.Params.Paused = true
			err := keeper.ValidateGenesis(gs)
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			require.Contains(t, err.Error(), tc.message)

			k, ctx, key := newKeeper(t)
			k.InitGenesis(ctx, *types.DefaultGenesis())
			before := storeContents(t, ctx.KVStore(key))
			panicErr := genesisPanic(t, func() { k.InitGenesis(ctx, gs) })
			require.ErrorIs(t, panicErr, types.ErrInvalidGenesis)
			require.EqualError(t, panicErr, err.Error())
			require.Equal(t, before, storeContents(t, ctx.KVStore(key)))
		})
	}
}

func TestValidateGenesisAcceptsExport(t *testing.T) {
	k, ctx, store := importGenesis(t, scenarioGenesis())
	exported := k.ExportGenesis(ctx)
	require.Len(t, exported.Orders, 10)
	require.Equal(t, types.OrderStatusResting, exported.Orders[0].Status)
	require.Len(t, exported.Positions, 4)
	require.Empty(t, exported.BookEntries)
	require.Empty(t, exported.TriggerEntries)
	require.Empty(t, exported.CloidEntries)
	require.Empty(t, exported.ExpiryEntries)
	require.Empty(t, exported.MarginRatioEntries)
	require.Empty(t, exported.AdlRankEntries)
	require.NoError(t, keeper.ValidateGenesis(*exported))

	restored, restoredCtx, restoredStore := importGenesis(t, *exported)
	require.Equal(t, storeContents(t, store), storeContents(t, restoredStore))
	require.Equal(t, exported, restored.ExportGenesis(restoredCtx))

	exported.BookEntries = []types.BookEntry{
		{MarketID: 0, Side: types.SideBuy, Price: genesisE18(59_000), Sequence: 1, OrderID: genesisWord(1)},
		{MarketID: 0, Side: types.SideSell, Price: genesisE18(61_000), Sequence: 2, OrderID: genesisWord(2)},
		{MarketID: 3, Side: types.SideBuy, Price: genesisE18(2_990), Sequence: 3, OrderID: genesisWord(3)},
		{MarketID: 7, Side: types.SideSell, Price: genesisE18(150), Sequence: 8, OrderID: genesisWord(8)},
	}
	require.NoError(t, keeper.ValidateGenesis(*exported))
}
