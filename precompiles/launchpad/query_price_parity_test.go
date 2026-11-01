package launchpad_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestPriceHistoryMatchesPriceSnapshots(t *testing.T) {
	e := newPoolEnv(t)
	router := baseapp.NewQueryServerTestHelper(e.ctx(), testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)
	want := make([]types.PricePoint, 0, types.SnapshotSlots+2)

	for i := 0; i < types.SnapshotSlots+2; i++ {
		height := int64((i + 1) * 10)
		now := genesisTime.Add(time.Duration(i) * time.Minute)
		e.stateDB.WithCtx(e.ctx().WithBlockHeight(height).WithBlockTime(now))
		e.evm.Context.BlockNumber = big.NewInt(height)
		e.evm.Context.Time = uint64(now.Unix())
		e.transfer(e.usdl, e.holder, e.pool, 1_000_000_000)
		e.write(e.holder, "swap", big.NewInt(1_000_000_000), big.NewInt(1), true,
			e.recipient, big.NewInt(genesisTime.Add(time.Hour).Unix()))

		price := e.view("getPrice").(*big.Int)
		want = append(want, types.PricePoint{Height: uint64(height), Price: sdk.NewIntFromBigInt(price)})
		retained := want
		if len(retained) > types.SnapshotSlots {
			retained = retained[len(retained)-types.SnapshotSlots:]
		}
		market, found := e.k.GetMarket(e.ctx(), e.denom)
		require.True(t, found)
		require.Equal(t, uint64(len(retained)), market.SnapshotCount)
		require.Equal(t, uint64((i+1)%types.SnapshotSlots), market.SnapshotIndex)
		require.Equal(t, new(big.Int).SetUint64(market.SnapshotCount), e.view("snapshotCount"))
		snapshots := e.view("getPriceSnapshots").([types.SnapshotSlots]*big.Int)

		router.Ctx = e.ctx()
		response, err := client.PriceHistory(context.Background(), &types.QueryPriceHistoryRequest{
			Denom: e.denom, FromHeight: 0, ToHeight: ^uint64(0),
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Points, int(market.SnapshotCount))
		require.Equal(t, retained, response.Points)
		for j, point := range response.Points {
			require.Zero(t, point.Price.BigInt().Cmp(snapshots[j]), "snapshot %d after swap %d", j, i+1)
		}
		for j := int(market.SnapshotCount); j < len(snapshots); j++ {
			require.Zero(t, snapshots[j].Sign(), "unused snapshot %d after swap %d", j, i+1)
		}
	}
}
