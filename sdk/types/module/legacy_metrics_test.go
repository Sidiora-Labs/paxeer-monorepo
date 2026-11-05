package module_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	oracletypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
)

func TestModuleEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx(nil)
	manager := module.NewManager(
		oracle.NewAppModule(testApp.AppCodec(), testApp.OracleKeeper, testApp.AccountKeeper, testApp.BankKeeper),
		pricefeed.NewAppModule(testApp.PricefeedKeeper),
	)
	manager.SetOrderMidBlockers(pricefeedtypes.ModuleName, oracletypes.ModuleName)
	require.Equal(t, []string{pricefeedtypes.ModuleName, oracletypes.ModuleName}, manager.OrderMidBlockers)

	sink := oteltest.InstallLegacySink(t)
	telemetry.MeasureSince(time.Now(), "module_test", "control")
	require.Contains(t, sink.Keys(), "oteltest.module_test.control")

	for _, height := range []int64{1, 2} {
		totalBefore := totalMidBlockDuration(reader)
		modulesBefore := midBlockDuration(reader)

		manager.MidBlock(ctx.WithBlockHeight(height), height)

		require.Equal(t, totalBefore+1, totalMidBlockDuration(reader),
			"height %d: record one total duration", height)
		require.Equal(t, modulesBefore+2, midBlockDuration(reader),
			"height %d: record one duration per ordered module", height)
		for _, key := range sink.Keys() {
			require.NotContains(t, key, ".module.total_mid_block",
				"height %d: no legacy total MidBlock timing", height)
			require.NotContains(t, key, ".module.mid_block",
				"height %d: no legacy per-module MidBlock timing", height)
		}
	}
}
