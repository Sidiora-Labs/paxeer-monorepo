package baseapp

import (
	"context"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

const baseappMeterName = "paxcosmos_baseapp"

func midBlockDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "mid_block_duration")
}

func endBlockDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "end_block_duration")
}

func processProposalDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "process_proposal_duration")
}

func finalizeBlockDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "finalize_block_duration")
}

func commitDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount(baseappMeterName, "commit_duration")
}

func TestBlockLifecycleRecordsDurations(t *testing.T) {
	reader := oteltest.Install()
	proposalKey := []byte("proposal-height")
	finalizedKey := []byte("finalized-height")
	app := setupBaseApp(t, func(app *BaseApp) {
		app.SetProcessProposalHandler(func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
			require.Equal(t, int64(1), req.Header.Height)
			require.Equal(t, req.Header.Height, ctx.BlockHeight())
			setIntOnStore(ctx.KVStore(capKey1), proposalKey, ctx.BlockHeight())
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		})
		app.SetFinalizeBlocker(func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
			require.Equal(t, int64(1), req.Header.Height)
			require.Equal(t, req.Header.Height, ctx.BlockHeight())
			events := app.MidBlock(ctx, req.Header.Height)
			setIntOnStore(ctx.KVStore(capKey1), finalizedKey, ctx.BlockHeight())
			end := app.EndBlock(ctx, abci.RequestEndBlock{Height: req.Header.Height})
			return &abci.ResponseFinalizeBlock{Events: append(events, end.Events...)}, nil
		})
	})
	ctx := context.Background()
	_, err := app.InitChain(ctx, &abci.RequestInitChain{ChainId: "otel-block-test"})
	require.NoError(t, err)

	instruments := []struct {
		name string
		read func(*oteltest.Reader) uint64
	}{
		{"mid_block_duration", midBlockDuration},
		{"end_block_duration", endBlockDuration},
		{"process_proposal_duration", processProposalDuration},
		{"finalize_block_duration", finalizeBlockDuration},
		{"commit_duration", commitDuration},
	}
	before := make(map[string]uint64, len(instruments))
	for _, instrument := range instruments {
		before[instrument.name] = instrument.read(reader)
	}

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
	require.Equal(t, int64(1), getIntFromStore(app.checkState.ctx.KVStore(capKey1), finalizedKey))

	for _, instrument := range instruments {
		require.Equal(t, uint64(1), instrument.read(reader)-before[instrument.name], instrument.name)
	}
}
