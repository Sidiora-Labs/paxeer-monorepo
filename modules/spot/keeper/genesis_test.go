package keeper_test

import (
	"testing"

	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtest "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestInitGenesisWritesActiveIndex(t *testing.T) {
	e := newLimitEnv(t)
	ids := make([]uint64, 3)
	for i := range ids {
		ids[i] = e.place(types.OrderTypeLimitBuy, 100_000_000, 500, 0)
	}
	source := e.k.ExportGenesis(e.ctx)
	require.Equal(t, ids, source.ActiveLimitOrders)
	require.NoError(t, source.Validate())

	app := testkeeper.EVMTestApp
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := pricefeedtest.WithPriceFeedStore(e.ctx, key)
	feed := pricefeedkeeper.NewKeeper(e.priceKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	k := keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, feed)
	k.InitGenesis(ctx, *source)

	count, err := k.GetActiveLimitOrderCount(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(3), count)
	id, err := k.ActiveLimitOrderID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, ids[1], id)
	require.Equal(t, source, k.ExportGenesis(ctx))
}
