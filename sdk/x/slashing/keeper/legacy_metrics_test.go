package keeper_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/testslashing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestSlashingEmitsNoLegacyTelemetry(t *testing.T) {
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

	assertNoLegacyTelemetry := func(t *testing.T, sink *oteltest.LegacySink) {
		t.Helper()
		for _, key := range sink.Keys() {
			require.NotContains(t, key, "begin_blocker")
			require.NotContains(t, key, "pax.cosmos.validator.slashed")
		}
	}

	t.Run("signed begin block", func(t *testing.T) {
		sink := oteltest.InstallLegacySink(t)
		blockCtx := ctx.WithBlockHeight(1)
		consAddr := sdk.ConsAddress(pks[0].Address())
		beforeInfo, found := app.SlashingKeeper.GetValidatorSigningInfo(blockCtx, consAddr)
		require.True(t, found)
		before := reader.HistogramCount("paxcosmos_x_slashing", "slashing_begin_blocker_duration")
		votes := testslashing.CreateBeginBlockReq(pks[0].Address(), power, true).LastCommitInfo.Votes
		require.Len(t, votes, 1)
		require.True(t, votes[0].SignedLastBlock)

		slashing.BeginBlocker(blockCtx, votes, app.SlashingKeeper)

		require.Equal(t, uint64(1), reader.HistogramCount("paxcosmos_x_slashing", "slashing_begin_blocker_duration")-before)
		afterInfo, found := app.SlashingKeeper.GetValidatorSigningInfo(blockCtx, consAddr)
		require.True(t, found)
		require.Equal(t, (beforeInfo.IndexOffset+1)%params.SignedBlocksWindow, afterInfo.IndexOffset)
		require.Zero(t, afterInfo.MissedBlocksCounter)
		assertNoLegacyTelemetry(t, sink)
	})

	t.Run("bonded double sign", func(t *testing.T) {
		sink := oteltest.InstallLegacySink(t)
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
		assertNoLegacyTelemetry(t, sink)
	})

	t.Run("downtime", func(t *testing.T) {
		sink := oteltest.InstallLegacySink(t)
		consAddr := sdk.ConsAddress(pks[1].Address())
		beforeValidator, found := app.StakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
		require.True(t, found)
		require.Equal(t, stakingtypes.Bonded, beforeValidator.Status)
		require.False(t, beforeValidator.Jailed)
		before := validatorSlashed(reader)
		votes := testslashing.CreateBeginBlockReq(pks[1].Address(), power, false).LastCommitInfo.Votes
		require.Len(t, votes, 1)
		require.False(t, votes[0].SignedLastBlock)

		for i := int64(0); i < 2; i++ {
			blockCtx := ctx.WithBlockHeight(params.SignedBlocksWindow + 1 + i)
			beforeDuration := reader.HistogramCount("paxcosmos_x_slashing", "slashing_begin_blocker_duration")

			slashing.BeginBlocker(blockCtx, votes, app.SlashingKeeper)

			require.Equal(t, uint64(1), reader.HistogramCount("paxcosmos_x_slashing", "slashing_begin_blocker_duration")-beforeDuration)
			require.Equal(t, i, validatorSlashed(reader)-before)
			assertNoLegacyTelemetry(t, sink)
		}

		afterValidator, found := app.StakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
		require.True(t, found)
		require.True(t, afterValidator.Jailed)
		require.True(t, afterValidator.Tokens.LT(beforeValidator.Tokens))
		signInfo, found := app.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
		require.True(t, found)
		require.Zero(t, signInfo.MissedBlocksCounter)
		require.Zero(t, signInfo.IndexOffset)
		require.Equal(t, ctx.BlockTime().Add(params.DowntimeJailDuration), signInfo.JailedUntil)
	})
}
