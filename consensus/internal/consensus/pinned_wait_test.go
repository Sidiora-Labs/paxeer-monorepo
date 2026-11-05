package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestPinnedHeightProposesWithoutMempoolTxs(t *testing.T) {
	for _, tc := range []struct {
		name              string
		createEmptyBlocks bool
		interval          time.Duration
	}{
		{name: "empty blocks disabled"},
		{name: "empty block interval", createEmptyBlocks: true, interval: time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cfg := configSetup(t)
			cfg.Consensus.CreateEmptyBlocks = tc.createEmptyBlocks
			cfg.Consensus.CreateEmptyBlocksInterval = tc.interval
			require.True(t, cfg.Consensus.WaitForTxs())
			cs, vss := makeState(ctx, t, makeStateArgs{config: cfg, validators: 1})
			key, err := vss[0].GetPubKey(ctx)
			require.NoError(t, err)
			const height int64 = 3
			pin, err := cstypes.NewLeaderPin(height, []crypto.PubKey{key})
			require.NoError(t, err)
			cs.SetLeaderPin(pin)

			newRoundCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewRound)
			proposalCh := subscribe(ctx, t, cs.eventBus, types.EventQueryCompleteProposal)
			blockCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewBlock)
			require.Zero(t, cs.txMempool.Size())
			cs.startTestRound(ctx, cs.roundState.Height(), 0)

			// Commit the genesis proof and the block signing its app hash first.
			for h := int64(1); h < height; h++ {
				ensureNewRound(t, newRoundCh, h, 0)
				ensureNewProposal(t, proposalCh, h, 0)
				ensureNewBlock(t, blockCh, h)
			}
			previous := cs.blockStore.LoadBlockMeta(height - 1)
			require.NotNil(t, previous)
			ensureNewRound(t, newRoundCh, height, 0)
			ensureNewProposal(t, proposalCh, height, 0)
			event, ok := ensureMessageBeforeTimeout(t, blockCh, ensureTimeout).Data().(types.EventDataNewBlock)
			require.True(t, ok)
			require.Equal(t, height, event.Block.Height)
			require.Equal(t, key.Address(), event.Block.ProposerAddress)
			require.Equal(t, previous.Header.AppHash, event.Block.AppHash, "the pinned block must not need an app-hash proof")
			require.Empty(t, event.Block.Data.Txs)
			require.Zero(t, cs.txMempool.Size())
		})
	}
}

func TestUnpinnedHeightWaitsForTxs(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cfg := configSetup(t)
	cfg.Consensus.CreateEmptyBlocks = false
	require.True(t, cfg.Consensus.WaitForTxs())
	cs, vss := makeState(ctx, t, makeStateArgs{config: cfg, validators: 1})
	key, err := vss[0].GetPubKey(ctx)
	require.NoError(t, err)
	const height int64 = 3
	pin, err := cstypes.NewLeaderPin(height+1, []crypto.PubKey{key})
	require.NoError(t, err)
	cs.SetLeaderPin(pin)

	newRoundCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewRound)
	proposalCh := subscribe(ctx, t, cs.eventBus, types.EventQueryCompleteProposal)
	blockCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewBlock)
	cs.startTestRound(ctx, cs.roundState.Height(), 0)
	for h := int64(1); h < height; h++ {
		ensureNewRound(t, newRoundCh, h, 0)
		ensureNewProposal(t, proposalCh, h, 0)
		ensureNewBlock(t, blockCh, h)
	}
	ensureNewRound(t, newRoundCh, height, 0)
	ensureNoNewEventOnChannel(t, proposalCh)
	rs := cs.GetRoundState()
	require.Equal(t, height, rs.Height)
	require.Equal(t, int32(0), rs.Round)
	require.Equal(t, cstypes.RoundStepNewRound, rs.Step)
	require.False(t, rs.LeaderPin.Applies(height))
	require.Zero(t, cs.txMempool.Size())

	tx := types.Tx("pinned-wait=value")
	response, err := cs.txMempool.CheckTx(ctx, tx)
	require.NoError(t, err)
	require.True(t, response.IsOK())
	ensureNewProposal(t, proposalCh, height, 0)
	event, ok := ensureMessageBeforeTimeout(t, blockCh, ensureTimeout).Data().(types.EventDataNewBlock)
	require.True(t, ok)
	require.Equal(t, height, event.Block.Height)
	require.Equal(t, types.Txs{tx}, event.Block.Data.Txs)
}
