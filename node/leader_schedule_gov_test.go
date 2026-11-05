package app_test

import (
	"context"
	"math"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/stretchr/testify/require"
)

func LeaderSchedule(t *testing.T, a *app.App, height, activation int64, want [][32]byte) {
	t.Helper()
	gotActivation, leaders, err := a.LeaderSchedule(height)
	require.NoError(t, err)
	require.Equal(t, activation, gotActivation)
	require.Equal(t, want, leaders)

	ctx, err := a.CreateQueryContext(0, false)
	require.NoError(t, err)
	pinned, err := a.SequencerKeeper.Leaders(ctx)
	require.NoError(t, err)
	require.Equal(t, pinned, leaders)
	require.Equal(t, seqkeeper.GetLeadersParam(ctx, a.GetSubspace(seqtypes.ModuleName)), leaders)
}

func TestLeaderScheduleFollowsGovernedChangeNextHeight(t *testing.T) {
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	const activation int64 = 1
	original := [][32]byte{{0: 3, 31: 4}, {0: 1, 31: 2}}
	governed := [][32]byte{original[1], {31: 5}, original[0]}
	paramSpace := a.GetSubspace(seqtypes.ModuleName)
	commitLeaderSourceBlock(t, a, func(ctx sdk.Context) {
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, activation))
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, original))
		require.NoError(t, seqkeeper.SetLeadersParam(ctx, paramSpace, original))
		require.NoError(t, a.SequencerKeeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{{
			KeyId: sequencerKeyID, PublicKey: sequencerPublicKey(),
			FirstHeight: uint64(activation), LastHeight: math.MaxUint64,
		}}))
	})
	commitBlock := func(write func(sdk.Context)) {
		t.Helper()
		next := a.LastBlockHeight() + 1
		batch := signedBatchAt(t, uint64(next), sequencerKeySeed, 1, 0,
			zeroSeqHash, zeroSeqHash, seqtypes.EmptyTxRoot[:], sequencerKeyID)
		resp, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: sequencedChainID, Height: next},
			Txs:    [][]byte{batch},
		})
		require.NoError(t, err)
		require.Len(t, resp.TxResults, 1)
		require.Zero(t, resp.TxResults[0].Code, resp.TxResults[0].Log)
		write(a.GetContextForDeliverTx(nil))
		a.SetDeliverStateToCommit()
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, next, a.LastBlockHeight())
	}

	height := a.LastBlockHeight() + 1
	LeaderSchedule(t, a, height, activation, original)
	value, err := codec.NewLegacyAmino().MarshalAsJSON(seqkeeper.LeaderList{
		governed[0][:], governed[1][:], governed[2][:],
	})
	require.NoError(t, err)
	change := proposal.NewParameterChangeProposal("Sequencer leaders", "Rotate the primary and standby list", []proposal.ParamChange{
		proposal.NewParamChange(seqtypes.ModuleName, string(seqkeeper.KeyLeaders), string(value)),
	}, false)
	require.NoError(t, change.ValidateBasic())

	commitBlock(func(ctx sdk.Context) {
		require.Equal(t, height, ctx.BlockHeight())
		stored, err := a.GovKeeper.SubmitProposal(ctx, change)
		require.NoError(t, err)
		pending, found := a.GovKeeper.GetProposal(ctx, stored.ProposalId)
		require.True(t, found)
		require.Equal(t, original, seqkeeper.GetLeadersParam(ctx, paramSpace))
		LeaderSchedule(t, a, height, activation, original)

		router := a.GovKeeper.Router()
		require.True(t, router.HasRoute(pending.ProposalRoute()))
		require.NoError(t, router.GetRoute(pending.ProposalRoute())(ctx, pending.GetContent()))
		require.Equal(t, governed, seqkeeper.GetLeadersParam(ctx, paramSpace))
		pinned, err := a.SequencerKeeper.Leaders(ctx)
		require.NoError(t, err)
		require.Equal(t, governed, pinned)
		LeaderSchedule(t, a, height, activation, original)
		LeaderSchedule(t, a, height+1, activation, original)
	})

	require.Equal(t, height, a.LastBlockHeight())
	LeaderSchedule(t, a, height+1, activation, governed)
	commitBlock(func(ctx sdk.Context) {
		require.Equal(t, height+1, ctx.BlockHeight())
		pinned, err := a.SequencerKeeper.Leaders(ctx)
		require.NoError(t, err)
		require.Equal(t, governed, pinned)
		LeaderSchedule(t, a, height+1, activation, governed)
	})
	LeaderSchedule(t, a, height+1, activation, governed)
}
