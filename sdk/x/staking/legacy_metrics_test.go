package staking_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
)

func TestStakingAbciEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	_, app, ctx := getBaseSimappWithCustomKeeper(t)
	ctx = ctx.WithBlockHeight(1)
	sink := oteltest.InstallLegacySink(t)
	require.Empty(t, sink.Keys())

	beginBefore := beginBlockerDuration(reader)
	endBefore := endBlockerDuration(reader)

	staking.BeginBlocker(ctx, app.StakingKeeper)

	require.Empty(t, sink.Keys(), "BeginBlocker must not emit legacy telemetry")
	require.Equal(t, beginBefore+1, beginBlockerDuration(reader),
		"BeginBlocker must still record one staking_begin_blocker_duration sample")
	require.Equal(t, endBefore, endBlockerDuration(reader))

	staking.EndBlocker(ctx, app.StakingKeeper)

	require.Empty(t, sink.Keys(), "EndBlocker must not emit legacy telemetry")
	require.Equal(t, endBefore+1, endBlockerDuration(reader),
		"EndBlocker must still record one staking_end_blocker_duration sample")
	require.Equal(t, beginBefore+1, beginBlockerDuration(reader))
}
