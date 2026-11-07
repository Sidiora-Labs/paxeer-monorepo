package staking_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
)

func beginBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_staking", "staking_begin_blocker_duration")
}

func endBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_staking", "staking_end_blocker_duration")
}

func TestBlockersRecordStakingDurations(t *testing.T) {
	reader := oteltest.Install()
	_, app, ctx := getBaseSimappWithCustomKeeper(t)
	ctx = ctx.WithBlockHeight(1)

	beginBefore := beginBlockerDuration(reader)
	endBefore := endBlockerDuration(reader)

	staking.BeginBlocker(ctx, app.StakingKeeper)

	require.Equal(t, beginBefore+1, beginBlockerDuration(reader),
		"BeginBlocker must record one staking_begin_blocker_duration sample")
	require.Equal(t, endBefore, endBlockerDuration(reader),
		"BeginBlocker must not record staking_end_blocker_duration")

	staking.EndBlocker(ctx, app.StakingKeeper)

	require.Equal(t, endBefore+1, endBlockerDuration(reader),
		"EndBlocker must record one staking_end_blocker_duration sample")
	require.Equal(t, beginBefore+1, beginBlockerDuration(reader),
		"EndBlocker must not record staking_begin_blocker_duration")
}
