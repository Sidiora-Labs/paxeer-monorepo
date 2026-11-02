package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func pendingPlanHeight(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt("paxcosmos_x_upgrade_keeper", "upgrade_pending_plan_height")
}

func TestScheduleUpgradeRecordsPendingPlanHeight(t *testing.T) {
	reader := oteltest.Install()
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	plan := types.Plan{Name: t.Name(), Height: 20}
	before, _ := pendingPlanHeight(reader)

	require.NoError(t, s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan))

	after, found := pendingPlanHeight(reader)
	require.True(t, found, "scheduling an upgrade must record its pending height")
	require.Equal(t, int64(20), after-before)
	stored, found := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
	require.True(t, found)
	require.Equal(t, plan, stored)
}
