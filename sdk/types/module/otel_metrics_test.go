package module_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	oracletypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
)

func totalMidBlockDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_types_module", "module_total_mid_block_duration")
}

func midBlockDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_types_module", "module_mid_block_duration")
}

func TestManagerMidBlockRecordsTotalAndPerModuleDuration(t *testing.T) {
	reader := oteltest.Install()
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx(nil)
	manager := module.NewManager(
		oracle.NewAppModule(testApp.AppCodec(), testApp.OracleKeeper, testApp.AccountKeeper, testApp.BankKeeper),
		pricefeed.NewAppModule(testApp.PricefeedKeeper),
	)
	manager.SetOrderMidBlockers(pricefeedtypes.ModuleName, oracletypes.ModuleName)
	require.Equal(t, []string{pricefeedtypes.ModuleName, oracletypes.ModuleName}, manager.OrderMidBlockers)

	for _, height := range []int64{1, 2} {
		totalBefore := totalMidBlockDuration(reader)
		modulesBefore := midBlockDuration(reader)

		manager.MidBlock(ctx.WithBlockHeight(height), height)

		require.Equal(t, totalBefore+1, totalMidBlockDuration(reader),
			"height %d: record one total duration beside the legacy total_mid_block emission", height)
		require.Equal(t, modulesBefore+2, midBlockDuration(reader),
			"height %d: record one duration per ordered module beside the legacy mid_block emission", height)
	}
}
