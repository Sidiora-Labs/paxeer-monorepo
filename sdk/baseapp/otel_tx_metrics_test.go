package baseapp

import (
	"context"
	"crypto/sha256"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func deliverTxDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "deliver_tx_duration")
}

func txCount(reader *oteltest.Reader) int64 {
	return reader.CounterSum(baseappMeterName, "tx")
}

func txResult(reader *oteltest.Reader) int64 {
	return reader.CounterSum(baseappMeterName, "tx_result")
}

func txGasUsed(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(baseappMeterName, "tx_gas_used")
}

func txGasWanted(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(baseappMeterName, "tx_gas_wanted")
}

func TestDeliverTxRecordsTxCountResultAndGas(t *testing.T) {
	reader := oteltest.Install()
	const gasLimit = uint64(1_000_000)
	anteKey := []byte("otel-ante-counter")
	deliverKey := []byte("otel-deliver-counter")
	app := setupBaseApp(t, func(app *BaseApp) {
		ante := anteHandlerTxTest(t, capKey1, anteKey)
		app.SetAnteHandler(func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			return ante(ctx.WithGasMeter(sdk.NewGasMeterWithMultiplier(ctx, gasLimit)), tx, simulate)
		})
		app.Router().AddRoute(sdk.NewRoute(routeMsgCounter, handlerMsgCounter(t, capKey1, deliverKey)))
	})
	_, err := app.InitChain(context.Background(), &abci.RequestInitChain{})
	require.NoError(t, err)
	app.setDeliverState(tmproto.Header{Height: 1})
	cdc := codec.NewLegacyAmino()
	registerTestCodec(cdc)
	txBytes, err := cdc.Marshal(newTxCounter(0, 0))
	require.NoError(t, err)
	tx, err := app.txDecoder(txBytes)
	require.NoError(t, err)

	durationBefore := deliverTxDuration(reader)
	countBefore := txCount(reader)
	resultBefore := txResult(reader)
	gasUsedBefore, _ := txGasUsed(reader)
	gasWantedBefore, _ := txGasWanted(reader)

	res := app.DeliverTx(app.deliverState.ctx, abci.RequestDeliverTxV2{Tx: txBytes}, tx, sha256.Sum256(txBytes))
	require.True(t, res.IsOK(), res.Log)
	require.Equal(t, int64(gasLimit), res.GasWanted)
	require.Positive(t, res.GasUsed)
	require.Equal(t, int64(1), getIntFromStore(app.deliverState.ctx.KVStore(capKey1), anteKey))
	require.Equal(t, int64(1), getIntFromStore(app.deliverState.ctx.KVStore(capKey1), deliverKey))

	require.Equal(t, uint64(1), deliverTxDuration(reader)-durationBefore)
	require.Equal(t, int64(1), txCount(reader)-countBefore)
	require.Equal(t, int64(1), txResult(reader)-resultBefore)
	gasUsedAfter, found := txGasUsed(reader)
	require.True(t, found, "tx_gas_used")
	require.Equal(t, res.GasUsed-gasUsedBefore, gasUsedAfter-gasUsedBefore)
	gasWantedAfter, found := txGasWanted(reader)
	require.True(t, found, "tx_gas_wanted")
	require.Equal(t, res.GasWanted-gasWantedBefore, gasWantedAfter-gasWantedBefore)
}
