package perps_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestGovernedFeeTierView(t *testing.T) {
	env := newViewsEnv(t, nil)
	ctx := env.evm.StateDB.(*state.DBImpl).Ctx()
	volume := viewsWord(t, "2000000")
	require.NoError(t, env.keeper.RecordVolume(ctx, viewsA, viewsB, volume))

	env.expect("getFees", nil, viewsInt(450), viewsInt(150), viewsInt(0), viewsInt(0))
	env.expect("getFeeTier", []interface{}{viewsC}, uint8(0), viewsInt(450), viewsInt(150), viewsInt(0))
	env.expect("getFeeTier", []interface{}{viewsA}, uint8(0), viewsInt(450), viewsInt(150), volume.ToBig())
	env.expect("getFeeTier", []interface{}{viewsB}, uint8(0), viewsInt(450), viewsInt(-30), volume.ToBig())

	params, err := env.keeper.GetParams(ctx)
	require.NoError(t, err)
	params.FeeTiers = []perpstypes.FeeTier{
		{MinVolumeUsd: sdk.ZeroInt(), TakerFeePpm: 620, MakerFeePpm: 210},
		{MinVolumeUsd: sdk.NewIntWithDecimal(1_000_000, perpstypes.USDDecimals), TakerFeePpm: 510, MakerFeePpm: 170},
		{MinVolumeUsd: sdk.NewIntWithDecimal(10_000_000, perpstypes.USDDecimals), TakerFeePpm: 390, MakerFeePpm: 90},
	}
	params.RebateTiers = []perpstypes.RebateTier{
		{MinMakerSharePpm: 10_000, MakerFeePpm: -17},
		{MinMakerSharePpm: 40_000, MakerFeePpm: -41},
	}
	response, err := perpskeeper.NewMsgServerImpl(env.keeper).UpdateParams(sdk.WrapSDKContext(ctx), &perpstypes.MsgUpdateParams{
		Authority: env.keeper.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := env.keeper.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, params, stored)

	for _, tc := range []struct {
		name    string
		account common.Address
		tier    uint8
		taker   uint64
		maker   int64
		volume  *big.Int
	}{
		{name: "zero volume", account: viewsC, tier: 0, taker: 620, maker: 210, volume: viewsInt(0)},
		{name: "volume tier", account: viewsA, tier: 1, taker: 510, maker: 170, volume: volume.ToBig()},
		{name: "maker rebate", account: viewsB, tier: 1, taker: 510, maker: -41, volume: volume.ToBig()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tier, err := env.keeper.FeeTier(ctx, tc.account)
			require.NoError(t, err)
			require.Equal(t, tc.tier, tier.Tier)
			require.Equal(t, tc.taker, tier.TakerFeePpm)
			require.Equal(t, tc.maker, tier.MakerFeePpm)
			require.Zero(t, tc.volume.Cmp(tier.Volume14dUsd.ToBig()))

			result, err := env.call("getFeeTier", tc.account)
			require.NoError(t, err)
			unpacked, err := env.abi.Methods["getFeeTier"].Outputs.Unpack(result)
			require.NoError(t, err)
			require.Len(t, unpacked, 4)
			require.Equal(t, tier.Tier, unpacked[0])
			for i, want := range []*big.Int{
				new(big.Int).SetUint64(tier.TakerFeePpm), big.NewInt(tier.MakerFeePpm), tier.Volume14dUsd.ToBig(),
			} {
				require.Zero(t, want.Cmp(unpacked[i+1].(*big.Int)), "getFeeTier output %d", i+1)
			}
		})
	}

	tierZero, err := env.keeper.FeeTier(ctx, viewsC)
	require.NoError(t, err)
	result, err := env.call("getFees")
	require.NoError(t, err)
	unpacked, err := env.abi.Methods["getFees"].Outputs.Unpack(result)
	require.NoError(t, err)
	require.Len(t, unpacked, 4)
	for i, want := range []*big.Int{
		new(big.Int).SetUint64(tierZero.TakerFeePpm), big.NewInt(tierZero.MakerFeePpm), viewsInt(0),
		new(big.Int).SetUint64(params.InsuranceSharePpm),
	} {
		require.Zero(t, want.Cmp(unpacked[i].(*big.Int)), "getFees output %d", i)
	}
}
