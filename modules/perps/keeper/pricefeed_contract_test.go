package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestPricefeedKeeperSatisfiesPerpsContract(t *testing.T) {
	key := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_pricefeed"))
	feed := pricefeedkeeper.NewKeeper(key, nil, nil, nil)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))

	var contract types.PricefeedKeeper = feed
	symbol, enabled, err := contract.MarketSymbol(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "BTC", symbol)
	require.True(t, enabled)

	price, err := contract.GetPrice(ctx, 0)
	require.NoError(t, err)
	require.True(t, price.Index.IsZero())
	require.True(t, price.Mark.IsZero())
	require.False(t, price.Fresh)
}
