package baseapp

import (
	"context"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func abciQueryDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "abci_query_duration")
}

func getTxPriorityHintDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "get_tx_priority_hint_duration")
}

func TestQueryAndPriorityHintRecordDurations(t *testing.T) {
	reader := oteltest.Install()
	app := setupBaseApp(t, func(app *BaseApp) {
		app.SetVersion("otel-query-test-version")
		app.SetTxPrioritizer(func(ctx sdk.Context, tx sdk.Tx) (int64, error) {
			require.True(t, ctx.IsCheckTx())
			return tx.(txTest).Counter, nil
		})
	})
	ctx := context.Background()
	_, err := app.InitChain(ctx, &abci.RequestInitChain{})
	require.NoError(t, err)
	cdc := codec.NewLegacyAmino()
	registerTestCodec(cdc)
	txBytes, err := cdc.Marshal(newTxCounter(17, 0))
	require.NoError(t, err)

	queryBefore := abciQueryDuration(reader)
	priorityBefore := getTxPriorityHintDuration(reader)

	query, err := app.Query(ctx, &abci.RequestQuery{Path: "app/version"})
	require.NoError(t, err)
	require.NotNil(t, query)
	require.True(t, query.IsOK(), query.Log)
	require.Equal(t, "otel-query-test-version", string(query.Value))
	priority, err := app.GetTxPriorityHint(ctx, &abci.RequestGetTxPriorityHintV2{Tx: txBytes})
	require.NoError(t, err)
	require.NotNil(t, priority)
	require.Equal(t, int64(17), priority.Priority)

	require.Equal(t, uint64(1), abciQueryDuration(reader)-queryBefore)
	require.Equal(t, uint64(1), getTxPriorityHintDuration(reader)-priorityBefore)
}
