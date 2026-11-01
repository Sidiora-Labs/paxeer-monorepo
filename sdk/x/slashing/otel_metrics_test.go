package slashing_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/testslashing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	"github.com/stretchr/testify/require"
)

func beginBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_slashing", "slashing_begin_blocker_duration")
}

func TestBeginBlockerRecordsSlashingBeginBlockerDuration(t *testing.T) {
	reader := oteltest.Install()
	app := paxapp.Setup(t, false, false, false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	pks := paxapp.CreateTestPubKeys(1)
	paxapp.AddTestAddrsFromPubKeys(app, ctx, pks, app.StakingKeeper.TokensFromConsensusPower(ctx, 200))
	tstaking := teststaking.NewHelper(t, ctx, app.StakingKeeper)
	const power = int64(100)
	tstaking.CreateValidatorWithValPower(sdk.ValAddress(pks[0].Address()), pks[0], power, true)
	staking.EndBlocker(ctx, app.StakingKeeper)
	consAddr := sdk.ConsAddress(pks[0].Address())
	votes := testslashing.CreateBeginBlockReq(pks[0].Address(), power, true).LastCommitInfo.Votes
	require.Len(t, votes, 1)
	require.True(t, votes[0].SignedLastBlock)

	for height := int64(1); height <= 3; height++ {
		ctx = ctx.WithBlockHeight(height)
		beforeInfo, found := app.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
		require.True(t, found)
		before := beginBlockerDuration(reader)

		slashing.BeginBlocker(ctx, votes, app.SlashingKeeper)

		require.Equal(t, uint64(1), beginBlockerDuration(reader)-before)
		afterInfo, found := app.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
		require.True(t, found)
		require.Equal(t, (beforeInfo.IndexOffset+1)%app.SlashingKeeper.SignedBlocksWindow(ctx), afterInfo.IndexOffset)
		require.Zero(t, afterInfo.MissedBlocksCounter)
	}
}
