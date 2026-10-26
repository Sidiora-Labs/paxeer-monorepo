package bank_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func TestInitGenesisRecordsBankInitGenesisDuration(t *testing.T) {
	reader := oteltest.Install()
	a := app.Setup(t, false, false, false)
	ctx := a.BaseApp.NewContext(false, tmproto.Header{})
	cdc := a.AppCodec()
	am := bank.NewAppModule(cdc, a.BankKeeper, a.AccountKeeper)
	genesisState := types.DefaultGenesisState()
	genesisState.Params.SendEnabled = []*types.SendEnabled{{Denom: "ubank", Enabled: false}}
	data := cdc.MustMarshalJSON(genesisState)
	before := reader.HistogramCount("paxcosmos_x_bank", "init_genesis_duration")

	require.Empty(t, am.InitGenesis(ctx, cdc, data))

	require.Equal(t, before+1, reader.HistogramCount("paxcosmos_x_bank", "init_genesis_duration"))
	require.Equal(t, genesisState.Params, a.BankKeeper.GetParams(ctx))
}

func TestInitGenesisEmitsNoLegacyCrisisUnmarshalSample(t *testing.T) {
	reader := oteltest.Install()
	a := app.Setup(t, false, false, false)
	ctx := a.BaseApp.NewContext(false, tmproto.Header{})
	cdc := a.AppCodec()
	am := bank.NewAppModule(cdc, a.BankKeeper, a.AccountKeeper)
	data := cdc.MustMarshalJSON(types.DefaultGenesisState())
	sink := oteltest.InstallLegacySink(t)
	telemetry.MeasureSince(time.Now(), "bank", "otel_test", "control")
	require.Contains(t, sink.Keys(), "oteltest.bank.otel_test.control")
	before := reader.HistogramCount("paxcosmos_x_bank", "init_genesis_duration")

	require.Empty(t, am.InitGenesis(ctx, cdc, data))

	require.Equal(t, before+1, reader.HistogramCount("paxcosmos_x_bank", "init_genesis_duration"))
	require.NotContains(t, sink.Keys(), "oteltest.InitGenesis.crisis.unmarshal")
}
