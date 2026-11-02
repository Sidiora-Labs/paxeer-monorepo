package keeper_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func validatorSlashed(reader *oteltest.Reader) int64 {
	return reader.CounterSum("paxcosmos_x_slashing_keeper", "validator_slashed")
}

func TestSlashRecordsValidatorSlashed(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{Time: time.Unix(100, 0).UTC()})
	pks := paxapp.CreateTestPubKeys(2)
	paxapp.AddTestAddrsFromPubKeys(app, ctx, pks, app.StakingKeeper.TokensFromConsensusPower(ctx, 200))
	tstaking := teststaking.NewHelper(t, ctx, app.StakingKeeper)
	const power = int64(100)
	for _, pk := range pks {
		tstaking.CreateValidatorWithValPower(sdk.ValAddress(pk.Address()), pk, power, true)
	}
	staking.EndBlocker(ctx, app.StakingKeeper)

	params := app.SlashingKeeper.GetParams(ctx)
	params.SignedBlocksWindow = 2
	params.MinSignedPerWindow = sdk.NewDecWithPrec(5, 1)
	params.SlashFractionDoubleSign = sdk.NewDecWithPrec(5, 2)
	params.SlashFractionDowntime = sdk.NewDecWithPrec(1, 2)
	app.SlashingKeeper.SetParams(ctx, params)

	t.Run("bonded double sign", func(t *testing.T) {
		consAddr := sdk.ConsAddress(pks[0].Address())
		beforeValidator, found := app.StakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
		require.True(t, found)
		require.Equal(t, stakingtypes.Bonded, beforeValidator.Status)
		before := validatorSlashed(reader)

		app.SlashingKeeper.Slash(ctx, consAddr, params.SlashFractionDoubleSign, power, ctx.BlockHeight())

		require.Equal(t, int64(1), validatorSlashed(reader)-before)
		afterValidator, found := app.StakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
		require.True(t, found)
		require.True(t, afterValidator.Tokens.LT(beforeValidator.Tokens))
	})

	t.Run("downtime", func(t *testing.T) {
		consAddr := sdk.ConsAddress(pks[1].Address())
		beforeValidator, found := app.StakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
		require.True(t, found)
		require.Equal(t, stakingtypes.Bonded, beforeValidator.Status)
		require.False(t, beforeValidator.Jailed)
		before := validatorSlashed(reader)

		for i := int64(0); i < 2; i++ {
			blockCtx := ctx.WithBlockHeight(params.SignedBlocksWindow + 1 + i)
			addr, missedInfo, signInfo, shouldSlash, slashInfo := app.SlashingKeeper.HandleValidatorSignatureConcurrent(blockCtx, pks[1].Address(), power, false)
			require.Equal(t, consAddr, addr)
			require.Equal(t, i == 1, shouldSlash)
			require.Equal(t, before, validatorSlashed(reader))
			if shouldSlash {
				require.Equal(t, int64(2), signInfo.MissedBlocksCounter)
				signInfo = app.SlashingKeeper.SlashJailAndUpdateSigningInfo(blockCtx, consAddr, slashInfo, signInfo)
				require.Equal(t, int64(0), signInfo.MissedBlocksCounter)
				require.Equal(t, int64(0), signInfo.IndexOffset)
				require.Equal(t, blockCtx.BlockTime().Add(params.DowntimeJailDuration), signInfo.JailedUntil)
			}
			app.SlashingKeeper.SetValidatorMissedBlocks(blockCtx, consAddr, missedInfo)
			app.SlashingKeeper.SetValidatorSigningInfo(blockCtx, consAddr, signInfo)
		}

		require.Equal(t, int64(1), validatorSlashed(reader)-before)
		afterValidator, found := app.StakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
		require.True(t, found)
		require.True(t, afterValidator.Jailed)
		require.True(t, afterValidator.Tokens.LT(beforeValidator.Tokens))
	})
}
