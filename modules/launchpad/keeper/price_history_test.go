package keeper_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestPriceHistoryRange(t *testing.T) {
	e := newEnv(t)
	creator := e.account(2_000_000)
	market := e.create(creator, types.FeeStrategyClaim)
	other := e.create(creator, types.FeeStrategyClaim)
	trader := e.account(100_000_000)
	const maxHeight = ^uint64(0)

	points, err := e.k.PriceHistory(e.ctx, market.Denom, 0, maxHeight)
	require.NoError(t, err)
	require.Empty(t, points)

	want := make([]types.PricePoint, 0, types.SnapshotSlots+2)
	for n := 0; n < types.SnapshotSlots+2; n++ {
		height := uint64((n + 1) * 10)
		e.ctx = e.ctx.WithBlockHeight(int64(height))
		e.at(mature + time.Duration(n)*time.Minute)
		e.swap(trader, market.Denom, true, 2_000_000)
		stored := e.market(market.Denom)
		price, err := stored.Price()
		require.NoError(t, err)
		want = append(want, types.PricePoint{Height: height, Price: sdk.NewIntFromBigInt(price)})
		retained := want
		if len(retained) > types.SnapshotSlots {
			retained = retained[len(retained)-types.SnapshotSlots:]
		}
		require.Equal(t, uint64(len(retained)), stored.SnapshotCount)
		points, err = e.k.PriceHistory(e.ctx, market.Denom, 0, maxHeight)
		require.NoError(t, err)
		require.Equal(t, retained, points)
	}

	before := e.market(market.Denom)
	for _, tc := range []struct {
		name     string
		from, to uint64
		want     []types.PricePoint
	}{
		{name: "inclusive range", from: 40, to: 60, want: want[3:6]},
		{name: "single height", from: 50, to: 50, want: want[4:5]},
		{name: "between heights", from: 41, to: 59, want: want[4:5]},
		{name: "evicted heights", from: 0, to: 20, want: []types.PricePoint{}},
		{name: "after latest", from: 101, to: maxHeight, want: []types.PricePoint{}},
		{name: "zero height", from: 0, to: 0, want: []types.PricePoint{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points, err := e.k.PriceHistory(e.ctx, market.Denom, tc.from, tc.to)
			require.NoError(t, err)
			require.Equal(t, tc.want, points)
		})
	}
	require.Equal(t, before, e.market(market.Denom))
	points, err = e.k.PriceHistory(e.ctx, other.Denom, 0, maxHeight)
	require.NoError(t, err)
	require.Empty(t, points)
}

func TestPriceHistoryRefusals(t *testing.T) {
	e := newEnv(t)
	market := e.create(e.account(1_000_000), types.FeeStrategyClaim)
	before := e.market(market.Denom)
	for _, denom := range []string{"unknown-price-market", ""} {
		points, err := e.k.PriceHistory(e.ctx, denom, 0, ^uint64(0))
		require.ErrorIs(t, err, types.ErrUnknownMarket)
		require.ErrorContains(t, err, ": "+denom)
		require.Nil(t, points)
	}
	for _, bounds := range [][2]uint64{{2, 1}, {1, 0}, {^uint64(0), 0}} {
		points, err := e.k.PriceHistory(e.ctx, market.Denom, bounds[0], bounds[1])
		require.ErrorIs(t, err, types.ErrInvalidRange)
		require.Nil(t, points)
	}
	require.Equal(t, before, e.market(market.Denom))
}
