package keeper

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var adlCounterpartiesPerBlock = int(types.DefaultParams().AdlPositionsPerBlock)

func TestExecuteADLHonoursAdlPositionsPerBlock(t *testing.T) {
	for _, bound := range []uint64{1, 2, types.DefaultParams().AdlPositionsPerBlock, 65, 1<<32 + 2} {
		t.Run(fmt.Sprintf("bound_%d", bound), func(t *testing.T) {
			k, ctx := adlTestKeeper(t)
			params := types.DefaultParams()
			params.AdlPositionsPerBlock = bound
			require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
			count := min(bound+2, 67)
			deficit := adlPositionFixture(999, true, adlUSD(count), adlUSD(70_000), uint256.Int{})
			adlSeedPosition(t, k, ctx, deficit, uint256.Int{}, adlUSD(2_000))
			for id := uint64(1); id <= count; id++ {
				position := adlPositionFixture(id, false, adlUSD(1), adlUSD(66_000), adlUSD(count+1-id))
				adlSeedPosition(t, k, ctx, position, *uint256.NewInt(12_000_000_000), uint256.Int{})
			}
			marks := map[uint64]uint256.Int{0: adlUSD(60_000)}
			events, err := k.ExecuteADL(ctx, []uint256.Int{deficit.ID}, marks)
			require.NoError(t, err)
			want := min(bound, count)
			matches := adlEvents(events, types.EventADLMatched)
			require.Len(t, matches, int(want))
			for id := uint64(1); id <= count; id++ {
				position, err := adlPosition(ctx.KVStore(k.storeKey), *uint256.NewInt(id))
				if id <= want {
					require.ErrorIs(t, err, types.ErrUnknownPosition)
					require.Equal(t, uint256.NewInt(id).ToBig(), matches[id-1].Args[1])
				} else {
					require.NoError(t, err)
					require.Equal(t, adlUSD(1), position.SizeBase)
				}
			}
			position, err := adlPosition(ctx.KVStore(k.storeKey), deficit.ID)
			if want == count {
				require.ErrorIs(t, err, types.ErrUnknownPosition)
				return
			}
			require.NoError(t, err)
			require.Equal(t, adlUSD(count-want), position.SizeBase)
			params.AdlPositionsPerBlock = 1
			require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
			events, err = k.ExecuteADL(ctx.WithBlockHeight(ctx.BlockHeight()+1), []uint256.Int{deficit.ID}, marks)
			require.NoError(t, err)
			require.Len(t, adlEvents(events, types.EventADLMatched), 1)
			position, err = adlPosition(ctx.KVStore(k.storeKey), deficit.ID)
			require.NoError(t, err)
			require.Equal(t, adlUSD(count-want-1), position.SizeBase)
		})
	}

	t.Run("shared_across_deficits", func(t *testing.T) {
		k, ctx := adlTestKeeper(t)
		params := types.DefaultParams()
		params.AdlPositionsPerBlock = 2
		require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
		first := adlPositionFixture(998, true, adlUSD(1), adlUSD(70_000), uint256.Int{})
		second := adlPositionFixture(999, true, adlUSD(3), adlUSD(70_000), uint256.Int{})
		adlSeedPosition(t, k, ctx, first, uint256.Int{}, adlUSD(2_000))
		adlSeedPosition(t, k, ctx, second, uint256.Int{}, adlUSD(2_000))
		for id := uint64(1); id <= 4; id++ {
			position := adlPositionFixture(id, false, adlUSD(1), adlUSD(66_000), adlUSD(5-id))
			adlSeedPosition(t, k, ctx, position, *uint256.NewInt(12_000_000_000), uint256.Int{})
		}
		events, err := k.ExecuteADL(ctx, []uint256.Int{first.ID, second.ID}, map[uint64]uint256.Int{0: adlUSD(60_000)})
		require.NoError(t, err)
		require.Len(t, adlEvents(events, types.EventADLMatched), 2)
		require.Len(t, adlEvents(events, types.EventADLExecuted), 2)
		_, err = adlPosition(ctx.KVStore(k.storeKey), first.ID)
		require.ErrorIs(t, err, types.ErrUnknownPosition)
		remaining, err := adlPosition(ctx.KVStore(k.storeKey), second.ID)
		require.NoError(t, err)
		require.Equal(t, adlUSD(2), remaining.SizeBase)
		for id := uint64(3); id <= 4; id++ {
			position, err := adlPosition(ctx.KVStore(k.storeKey), *uint256.NewInt(id))
			require.NoError(t, err)
			require.Equal(t, adlUSD(1), position.SizeBase)
		}
	})

	t.Run("corrupt_params", func(t *testing.T) {
		k, ctx := adlTestKeeper(t)
		ctx.KVStore(k.storeKey).Set(types.ParamsKey, []byte("{"))
		events, err := k.ExecuteADL(ctx, nil, nil)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Empty(t, events)
	})
}

func TestRankADLLimitCappedByParams(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bound uint64
		limit uint32
		want  int
	}{
		{"zero_limit", 2, 0, 0},
		{"below_bound", 2, 1, 1},
		{"at_bound", 2, 2, 2},
		{"above_bound", 2, 3, 2},
		{"above_old_constant", 2, 65, 2},
		{"default", types.DefaultParams().AdlPositionsPerBlock, 66, 64},
		{"raised_bound", 65, 66, 65},
		{"uint64_bound", 1<<32 + 2, 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := adlTestKeeper(t)
			if tc.name == "default" {
				ctx.KVStore(k.storeKey).Delete(types.ParamsKey)
			} else {
				params := types.DefaultParams()
				params.AdlPositionsPerBlock = tc.bound
				require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
			}
			deficit := adlPositionFixture(999, true, adlUSD(67), adlUSD(70_000), uint256.Int{})
			adlSeedPosition(t, k, ctx, deficit, uint256.Int{}, adlUSD(2_000))
			for id := uint64(1); id <= 67; id++ {
				position := adlPositionFixture(id, false, adlUSD(1), adlUSD(66_000), adlUSD(68-id))
				adlSeedPosition(t, k, ctx, position, *uint256.NewInt(12_000_000_000), uint256.Int{})
			}
			positions, err := k.RankADL(ctx, deficit.ID, adlUSD(60_000), tc.limit)
			require.NoError(t, err)
			require.Len(t, positions, tc.want)
			for i, position := range positions {
				require.Equal(t, *uint256.NewInt(uint64(i + 1)), position.ID)
				require.Equal(t, adlUSD(1), position.SizeBase)
			}
		})
	}

	t.Run("corrupt_params", func(t *testing.T) {
		k, ctx := adlTestKeeper(t)
		ctx.KVStore(k.storeKey).Set(types.ParamsKey, []byte("{"))
		positions, err := k.RankADL(ctx, *uint256.NewInt(999), adlUSD(60_000), 1)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Empty(t, positions)
	})
}
