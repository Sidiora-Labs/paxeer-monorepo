package upgrade_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func beginBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_upgrade", "begin_blocker_duration")
}

func planHeight(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt("paxcosmos_x_upgrade", "plan_height")
}

func TestBeginBlockerRecordsUpgradeDurationAndPlanHeight(t *testing.T) {
	reader := oteltest.Install()
	s := setupTest(t, 10)
	plan := types.Plan{Name: t.Name(), Height: 20}
	require.NoError(t, s.keeper.ScheduleUpgrade(s.ctx, plan))
	beforeDuration := beginBlockerDuration(reader)
	beforeHeight, _ := planHeight(reader)

	upgrade.BeginBlocker(s.keeper, s.ctx)

	afterDuration := beginBlockerDuration(reader)
	afterHeight, found := planHeight(reader)
	require.Equal(t, beforeDuration+1, afterDuration, "BeginBlocker must record one duration observation")
	require.True(t, found, "BeginBlocker must record the scheduled plan height")
	require.Equal(t, int64(20), afterHeight-beforeHeight)
	stored, found := s.keeper.GetUpgradePlan(s.ctx)
	require.True(t, found)
	require.Equal(t, plan, stored)
}
