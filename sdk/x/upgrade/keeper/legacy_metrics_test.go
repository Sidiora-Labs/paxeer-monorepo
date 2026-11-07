package keeper_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestUpgradeEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	plan := types.Plan{Name: t.Name(), Height: 20, Info: "upgrade telemetry regression"}

	sink := oteltest.InstallLegacySink(t)
	telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), "otel_test", "duration_control")
	telemetry.SetGaugeWithLabels([]string{"otel_test", "gauge_control"}, 1, nil)
	controlKeys := []string{"oteltest.otel_test.duration_control", "oteltest.otel_test.gauge_control"}
	require.Equal(t, controlKeys, sink.Keys(), "the legacy sink must capture both samples and gauges")
	beforePendingHeight, _ := pendingPlanHeight(reader)

	require.NoError(t, s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan))

	afterPendingHeight, found := pendingPlanHeight(reader)
	require.True(t, found)
	require.Equal(t, plan.Height, afterPendingHeight-beforePendingHeight)
	require.Equal(t, controlKeys, sink.Keys(), "ScheduleUpgrade must not emit legacy telemetry")
	beforeDuration := reader.HistogramCount("paxcosmos_x_upgrade", "begin_blocker_duration")
	beforePlanHeight, _ := reader.GaugeInt("paxcosmos_x_upgrade", "plan_height")

	upgrade.BeginBlocker(s.app.UpgradeKeeper, s.ctx)

	require.Equal(t, beforeDuration+1, reader.HistogramCount("paxcosmos_x_upgrade", "begin_blocker_duration"))
	afterPlanHeight, found := reader.GaugeInt("paxcosmos_x_upgrade", "plan_height")
	require.True(t, found)
	require.Equal(t, plan.Height, afterPlanHeight-beforePlanHeight)
	require.Equal(t, controlKeys, sink.Keys(), "BeginBlocker must not emit legacy telemetry")
	stored, found := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
	require.True(t, found)
	require.Equal(t, plan, stored)
}
