package baseapp

import (
	"context"
	"crypto/sha256"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestBaseappAbciEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	sink := oteltest.InstallLegacySink(t)
	telemetry.IncrCounter(1, "legacy_abci_test", "control")
	require.Equal(t, []string{"oteltest.legacy_abci_test.control"}, sink.Keys())

	const gasLimit = uint64(1_000_000)
	anteKey := []byte("legacy-abci-ante-counter")
	deliverKey := []byte("legacy-abci-deliver-counter")
	proposalKey := []byte("legacy-abci-proposal-height")
	cdc := codec.NewLegacyAmino()
	registerTestCodec(cdc)
	txBytes, err := cdc.Marshal(newTxCounter(0, 0))
	require.NoError(t, err)
	failingTx := newTxCounter(1, 1)
	failingTx.FailOnAnte = true
	failedTxBytes, err := cdc.Marshal(failingTx)
	require.NoError(t, err)
	var successful, failed abci.ResponseDeliverTx
	app := setupBaseApp(t, func(app *BaseApp) {
		app.SetVersion("legacy-abci-test-version")
		ante := anteHandlerTxTest(t, capKey1, anteKey)
		app.SetAnteHandler(func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			return ante(ctx.WithGasMeter(sdk.NewGasMeterWithMultiplier(ctx, gasLimit)), tx, simulate)
		})
		app.Router().AddRoute(sdk.NewRoute(routeMsgCounter, handlerMsgCounter(t, capKey1, deliverKey)))
		app.SetTxPrioritizer(func(ctx sdk.Context, tx sdk.Tx) (int64, error) {
			require.True(t, ctx.IsCheckTx())
			return tx.(txTest).Counter, nil
		})
		app.SetProcessProposalHandler(func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
			require.Equal(t, int64(1), ctx.BlockHeight())
			setIntOnStore(ctx.KVStore(capKey1), proposalKey, req.Header.Height)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		})
		app.SetFinalizeBlocker(func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
			require.Equal(t, int64(1), ctx.BlockHeight())
			events := app.MidBlock(ctx, req.Header.Height)
			tx, err := app.txDecoder(txBytes)
			require.NoError(t, err)
			successful = app.DeliverTx(ctx, abci.RequestDeliverTxV2{Tx: txBytes}, tx, sha256.Sum256(txBytes))
			require.True(t, successful.IsOK(), successful.Log)
			require.Equal(t, int64(gasLimit), successful.GasWanted)
			require.Positive(t, successful.GasUsed)
			failedTx, err := app.txDecoder(failedTxBytes)
			require.NoError(t, err)
			failed = app.DeliverTx(ctx, abci.RequestDeliverTxV2{Tx: failedTxBytes}, failedTx, sha256.Sum256(failedTxBytes))
			require.False(t, failed.IsOK(), "the transaction must fail ante validation")
			end := app.EndBlock(ctx, abci.RequestEndBlock{Height: req.Header.Height})
			return &abci.ResponseFinalizeBlock{Events: append(events, end.Events...)}, nil
		})
	})
	ctx := context.Background()
	_, err = app.InitChain(ctx, &abci.RequestInitChain{ChainId: "legacy-abci-test"})
	require.NoError(t, err)

	durations := []struct {
		name string
		read func(*oteltest.Reader) uint64
		want uint64
	}{
		{"mid_block_duration", midBlockDuration, 1},
		{"end_block_duration", endBlockDuration, 1},
		{"process_proposal_duration", processProposalDuration, 1},
		{"finalize_block_duration", finalizeBlockDuration, 1},
		{"commit_duration", commitDuration, 1},
		{"deliver_tx_duration", deliverTxDuration, 2},
		{"abci_query_duration", abciQueryDuration, 1},
		{"get_tx_priority_hint_duration", getTxPriorityHintDuration, 1},
	}
	before := make(map[string]uint64, len(durations))
	for _, duration := range durations {
		before[duration.name] = duration.read(reader)
	}
	countBefore := txCount(reader)
	resultBefore := txResult(reader)

	query, err := app.Query(ctx, &abci.RequestQuery{Path: "app/version"})
	require.NoError(t, err)
	require.NotNil(t, query)
	require.True(t, query.IsOK(), query.Log)
	require.Equal(t, "legacy-abci-test-version", string(query.Value))
	priorityBytes, err := cdc.Marshal(newTxCounter(17, 0))
	require.NoError(t, err)
	priority, err := app.GetTxPriorityHint(ctx, &abci.RequestGetTxPriorityHintV2{Tx: priorityBytes})
	require.NoError(t, err)
	require.NotNil(t, priority)
	require.Equal(t, int64(17), priority.Priority)

	header := &tmproto.Header{ChainID: app.ChainID, Height: 1}
	proposal, err := app.ProcessProposal(ctx, &abci.RequestProcessProposal{Header: header})
	require.NoError(t, err)
	require.NotNil(t, proposal)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, proposal.Status)
	require.Equal(t, int64(1), getIntFromStore(app.processProposalState.ctx.KVStore(capKey1), proposalKey))
	finalized, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Header: header})
	require.NoError(t, err)
	require.NotNil(t, finalized)
	app.SetDeliverStateToCommit()
	committed, err := app.Commit(ctx)
	require.NoError(t, err)
	require.NotNil(t, committed)
	require.Equal(t, int64(1), app.LastBlockHeight())
	require.Equal(t, int64(1), getIntFromStore(app.checkState.ctx.KVStore(capKey1), anteKey))
	require.Equal(t, int64(1), getIntFromStore(app.checkState.ctx.KVStore(capKey1), deliverKey))

	for _, duration := range durations {
		require.Equal(t, duration.want, duration.read(reader)-before[duration.name], duration.name)
	}
	require.Equal(t, int64(2), txCount(reader)-countBefore)
	require.Equal(t, int64(2), txResult(reader)-resultBefore)
	gasUsed, found := txGasUsed(reader)
	require.True(t, found, "tx_gas_used")
	require.Equal(t, failed.GasUsed, gasUsed)
	gasWanted, found := txGasWanted(reader)
	require.True(t, found, "tx_gas_wanted")
	require.Equal(t, failed.GasWanted, gasWanted)

	keys := sink.Keys()
	require.Contains(t, keys, "oteltest.legacy_abci_test.control")
	for _, key := range []string{
		"abci.mid_block",
		"abci.end_block",
		"abci.deliver_tx",
		"tx.count",
		"tx.successful",
		"tx.failed",
		"tx.gas.used",
		"tx.gas.wanted",
		"abci.commit",
		"abci.query",
		"abci.process_proposal",
		"abci.finalize_block",
		"abci.get_tx_priority_hint",
	} {
		require.NotContains(t, keys, "oteltest."+key, "ABCI must not emit legacy telemetry for %s", key)
	}
}
