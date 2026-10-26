package types_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMarketPriceHistoryOrdersRing(t *testing.T) {
	const maxHeight = ^uint64(0)
	for count := uint64(0); count <= types.SnapshotSlots; count++ {
		for index := uint64(0); index < types.SnapshotSlots; index++ {
			t.Run(fmt.Sprintf("count_%d/index_%d", count, index), func(t *testing.T) {
				market := types.Market{
					PriceSnapshots:  make([]sdk.Int, types.SnapshotSlots),
					SnapshotHeights: make([]uint64, types.SnapshotSlots),
					SnapshotCount:   count,
					SnapshotIndex:   index,
				}
				for slot := range market.PriceSnapshots {
					market.PriceSnapshots[slot] = sdk.NewInt(999)
					market.SnapshotHeights[slot] = 999
				}
				want := make([]types.PricePoint, 0, count)
				for i := uint64(0); i < count; i++ {
					slot := i
					if count == types.SnapshotSlots {
						slot = (index + i) % types.SnapshotSlots
					}
					point := types.PricePoint{Height: 10 + i*10, Price: sdk.NewIntFromUint64(100 + i)}
					market.PriceSnapshots[slot] = point.Price
					market.SnapshotHeights[slot] = point.Height
					want = append(want, point)
				}
				points, err := market.PriceHistory(0, maxHeight)
				require.NoError(t, err)
				require.Equal(t, want, points)

				filtered := make([]types.PricePoint, 0)
				for _, point := range want {
					if point.Height >= 20 && point.Height <= 40 {
						filtered = append(filtered, point)
					}
				}
				points, err = market.PriceHistory(20, 40)
				require.NoError(t, err)
				require.Equal(t, filtered, points)

				points, err = market.PriceHistory(30, 30)
				require.NoError(t, err)
				if count >= 3 {
					require.Equal(t, want[2:3], points)
				} else {
					require.Empty(t, points)
				}
				points, err = market.PriceHistory(81, maxHeight)
				require.NoError(t, err)
				require.Empty(t, points)
			})
		}
	}

	t.Run("height boundaries and repeated heights", func(t *testing.T) {
		market := types.Market{
			PriceSnapshots:  []sdk.Int{sdk.NewInt(1), sdk.NewInt(2), sdk.NewInt(3), sdk.NewInt(4), sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt(), sdk.ZeroInt()},
			SnapshotHeights: []uint64{0, 7, 7, maxHeight, 0, 0, 0, 0},
			SnapshotCount:   4,
			SnapshotIndex:   4,
		}
		for _, tc := range []struct {
			height uint64
			want   []types.PricePoint
		}{
			{0, []types.PricePoint{{Height: 0, Price: sdk.NewInt(1)}}},
			{7, []types.PricePoint{{Height: 7, Price: sdk.NewInt(2)}, {Height: 7, Price: sdk.NewInt(3)}}},
			{maxHeight, []types.PricePoint{{Height: maxHeight, Price: sdk.NewInt(4)}}},
		} {
			points, err := market.PriceHistory(tc.height, tc.height)
			require.NoError(t, err)
			require.Equal(t, tc.want, points)
		}
	})
}

func TestMarketPriceHistoryRefusals(t *testing.T) {
	newMarket := func() types.Market {
		return types.Market{
			PriceSnapshots:  []sdk.Int{sdk.NewInt(81), sdk.NewInt(82), sdk.NewInt(83), sdk.NewInt(84), sdk.NewInt(85), sdk.NewInt(86), sdk.NewInt(87), sdk.NewInt(88)},
			SnapshotHeights: make([]uint64, types.SnapshotSlots),
			SnapshotCount:   3,
			SnapshotIndex:   3,
		}
	}
	t.Run("invalid range", func(t *testing.T) {
		points, err := newMarket().PriceHistory(2, 1)
		require.ErrorIs(t, err, types.ErrInvalidRange)
		require.Nil(t, points)
	})
	for name, mutate := range map[string]func(*types.Market){
		"nil prices":            func(m *types.Market) { m.PriceSnapshots = nil },
		"short prices":          func(m *types.Market) { m.PriceSnapshots = m.PriceSnapshots[:types.SnapshotSlots-1] },
		"long prices":           func(m *types.Market) { m.PriceSnapshots = append(m.PriceSnapshots, sdk.ZeroInt()) },
		"index at limit":        func(m *types.Market) { m.SnapshotIndex = types.SnapshotSlots },
		"index above limit":     func(m *types.Market) { m.SnapshotIndex = ^uint64(0) },
		"short heights":         func(m *types.Market) { m.SnapshotHeights = m.SnapshotHeights[:types.SnapshotSlots-1] },
		"long heights":          func(m *types.Market) { m.SnapshotHeights = append(m.SnapshotHeights, 0) },
		"count above limit":     func(m *types.Market) { m.SnapshotCount = types.SnapshotSlots + 1 },
		"count at uint64 limit": func(m *types.Market) { m.SnapshotCount = ^uint64(0) },
	} {
		t.Run(name, func(t *testing.T) {
			market := newMarket()
			mutate(&market)
			for _, bounds := range [][2]uint64{{0, 100}, {2, 1}} {
				points, err := market.PriceHistory(bounds[0], bounds[1])
				require.ErrorIs(t, err, types.ErrInvalidMarket)
				require.ErrorContains(t, err, "snapshot ring")
				require.Nil(t, points)
			}
		})
	}
	for _, emptyHeights := range [][]uint64{nil, {}} {
		for _, full := range []bool{false, true} {
			market := newMarket()
			market.SnapshotHeights = emptyHeights
			want := []types.PricePoint{
				{Height: 0, Price: sdk.NewInt(81)},
				{Height: 0, Price: sdk.NewInt(82)},
				{Height: 0, Price: sdk.NewInt(83)},
			}
			if full {
				market.SnapshotCount = types.SnapshotSlots
				want = []types.PricePoint{
					{Height: 0, Price: sdk.NewInt(84)},
					{Height: 0, Price: sdk.NewInt(85)},
					{Height: 0, Price: sdk.NewInt(86)},
					{Height: 0, Price: sdk.NewInt(87)},
					{Height: 0, Price: sdk.NewInt(88)},
					{Height: 0, Price: sdk.NewInt(81)},
					{Height: 0, Price: sdk.NewInt(82)},
					{Height: 0, Price: sdk.NewInt(83)},
				}
			}
			points, err := market.PriceHistory(0, 0)
			require.NoError(t, err)
			require.Equal(t, want, points)
			points, err = market.PriceHistory(1, ^uint64(0))
			require.NoError(t, err)
			require.Empty(t, points)
		}
	}
}
