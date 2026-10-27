package types_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRegistryInfoOfCopiesEveryField(t *testing.T) {
	t.Run("fee right", func(t *testing.T) {
		for _, strategy := range []types.FeeStrategy{
			types.FeeStrategyClaim,
			types.FeeStrategyBurn,
			types.FeeStrategyAirdrop,
			types.FeeStrategyLpRewards,
		} {
			right := types.FeeRight{
				ID:       1<<63 + 17,
				Owner:    "0x0000000000000000000000000000000000000001",
				Approved: "0x0000000000000000000000000000000000000002",
				Strategy: strategy,
				Pool:     "0x0000000000000000000000000000000000000003",
			}
			require.Equal(t, types.FeeRightInfo{
				ID:       1<<63 + 17,
				Owner:    "0x0000000000000000000000000000000000000001",
				Approved: "0x0000000000000000000000000000000000000002",
				Strategy: uint32(strategy),
				Pool:     "0x0000000000000000000000000000000000000003",
			}, types.FeeRightInfoOf(right))
		}
	})

	t.Run("airdrop epoch", func(t *testing.T) {
		epoch := types.PoolAirdropEpoch{
			Pool:            "0x0000000000000000000000000000000000000004",
			Epoch:           1<<63 + 19,
			Amount:          sdk.NewIntWithDecimal(101, 20),
			SnapshotBlock:   1<<63 + 23,
			EligibleSupply:  sdk.NewIntWithDecimal(103, 20),
			Remaining:       sdk.NewIntWithDecimal(107, 20),
			ClaimedSupply:   sdk.NewIntWithDecimal(109, 20),
			LegacyRoot:      "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			LegacyRemaining: sdk.NewIntWithDecimal(113, 20),
		}
		require.Equal(t, types.AirdropEpochInfo{
			Pool:            "0x0000000000000000000000000000000000000004",
			Epoch:           1<<63 + 19,
			Amount:          sdk.NewIntWithDecimal(101, 20),
			SnapshotBlock:   1<<63 + 23,
			EligibleSupply:  sdk.NewIntWithDecimal(103, 20),
			Remaining:       sdk.NewIntWithDecimal(107, 20),
			ClaimedSupply:   sdk.NewIntWithDecimal(109, 20),
			LegacyRoot:      "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			LegacyRemaining: sdk.NewIntWithDecimal(113, 20),
		}, types.AirdropEpochInfoOf(epoch))
	})

	t.Run("retired address", func(t *testing.T) {
		retired := types.RetiredAddress{
			Address:     "0x0000000000000000000000000000000000000005",
			Role:        types.RoleRouter,
			Replacement: "0x0000000000000000000000000000000000000006",
		}
		require.Equal(t, types.RetiredInfo{
			Address:     "0x0000000000000000000000000000000000000005",
			Role:        types.RoleRouter,
			Replacement: "0x0000000000000000000000000000000000000006",
		}, types.RetiredInfoOf(retired))
	})

	t.Run("zero values", func(t *testing.T) {
		require.Equal(t, types.FeeRightInfo{}, types.FeeRightInfoOf(types.FeeRight{}))
		require.Equal(t, types.AirdropEpochInfo{}, types.AirdropEpochInfoOf(types.PoolAirdropEpoch{}))
		require.Equal(t, types.RetiredInfo{}, types.RetiredInfoOf(types.RetiredAddress{}))
	})
}
