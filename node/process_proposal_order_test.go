package app_test

import (
	"bytes"
	"testing"
	"time"

	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func orderCheckBatch(t *testing.T, height, from, to uint64, prev, last []byte) seqtypes.MsgSequenceBatch {
	t.Helper()
	encoded := signedBatchAt(t, height, sequencerKeySeed, from, to, prev, last,
		seqtypes.EmptyTxRoot[:], sequencerKeyID)
	batch, err := app.DecodeSequenceBatch(encoded)
	require.NoError(t, err)
	return batch
}

func DeliverSequenceBatch(t *testing.T, w *app.TestWrapper, batch seqtypes.MsgSequenceBatch) {
	t.Helper()
	require.NoError(t, w.App.SequencerKeeper.DeliverSequenceBatch(w.Ctx, batch))
}

func newOrderCheckWrapper(t *testing.T) (*app.TestWrapper, []byte) {
	t.Helper()
	w := newSequencerWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	activate(t, w)
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
	w.Ctx = w.Ctx.WithChainID(sequencedChainID).WithBlockHeight(2).WithTxIndex(0)
	last := bytes.Repeat([]byte{0x77}, seqtypes.HashLength)
	initial := orderCheckBatch(t, 2, 1, 7, zeroSeqHash, last)
	DeliverSequenceBatch(t, w, initial)
	requireSequencerState(t, w, 7, last)
	w.Ctx = w.Ctx.WithBlockHeight(3)
	return w, last
}

func orderCheckStore(t *testing.T, w *app.TestWrapper, ctx sdk.Context) map[string][]byte {
	t.Helper()
	it := ctx.KVStore(w.App.GetKey(seqtypes.StoreKey)).Iterator(nil, nil)
	defer it.Close()
	state := make(map[string][]byte)
	for ; it.Valid(); it.Next() {
		state[string(it.Key())] = bytes.Clone(it.Value())
	}
	return state
}

func TestOrderCheckRunsBatchOnBranch(t *testing.T) {
	w, prev := newOrderCheckWrapper(t)
	last := bytes.Repeat([]byte{0x88}, seqtypes.HashLength)
	batch := orderCheckBatch(t, 3, 8, 9, prev, last)
	require.NoError(t, w.App.CheckSequencerOrder(w.Ctx, batch))
	requireSequencerState(t, w, 7, prev)

	badSignature := batch
	badSignature.Sig = bytes.Clone(batch.Sig)
	badSignature.Sig[0] ^= 1
	err := w.App.CheckSequencerOrder(w.Ctx, badSignature)
	require.ErrorIs(t, err, seqtypes.ErrInvalidSequenceBatch)
	require.Contains(t, err.Error(), seqtypes.ErrBadSignature.Error())
	requireSequencerState(t, w, 7, prev)

	empty := orderCheckBatch(t, 3, 8, 7, prev, prev)
	require.NoError(t, w.App.CheckSequencerOrder(w.Ctx, empty))
	requireSequencerState(t, w, 7, prev)
}

func TestOrderCheckRefusesSeqGap(t *testing.T) {
	w, prev := newOrderCheckWrapper(t)
	for _, from := range []uint64{7, 9} {
		batch := orderCheckBatch(t, 3, from, from, prev, bytes.Repeat([]byte{0x88}, seqtypes.HashLength))
		err := w.App.CheckSequencerOrder(w.Ctx, batch)
		require.ErrorIs(t, err, seqtypes.ErrInvalidSequenceBatch)
		require.Contains(t, err.Error(), "fromSeq")
		requireSequencerState(t, w, 7, prev)
	}
}

func TestOrderCheckRefusesPrevSeqHashMismatch(t *testing.T) {
	w, prev := newOrderCheckWrapper(t)
	wrong := bytes.Clone(prev)
	wrong[0] ^= 1
	batch := orderCheckBatch(t, 3, 8, 9, wrong, bytes.Repeat([]byte{0x88}, seqtypes.HashLength))
	err := w.App.CheckSequencerOrder(w.Ctx, batch)
	require.ErrorIs(t, err, seqtypes.ErrInvalidSequenceBatch)
	require.Contains(t, err.Error(), "prevSeqHash")
	requireSequencerState(t, w, 7, prev)
}

func TestOrderCheckLeavesStateUntouched(t *testing.T) {
	w, prev := newOrderCheckWrapper(t)
	before := orderCheckStore(t, w, w.Ctx)
	proposal, writeProposal := w.Ctx.CacheContext()
	last := bytes.Repeat([]byte{0x88}, seqtypes.HashLength)
	batch := orderCheckBatch(t, 3, 8, 9, prev, last)
	for i := 0; i < 2; i++ {
		require.NoError(t, w.App.CheckSequencerOrder(proposal, batch))
		require.Equal(t, before, orderCheckStore(t, w, proposal))
		require.Equal(t, before, orderCheckStore(t, w, w.Ctx))
	}
	gap := orderCheckBatch(t, 3, 10, 10, prev, last)
	require.ErrorIs(t, w.App.CheckSequencerOrder(proposal, gap), seqtypes.ErrInvalidSequenceBatch)
	require.Equal(t, before, orderCheckStore(t, w, proposal))
	writeProposal()
	require.Equal(t, before, orderCheckStore(t, w, w.Ctx))
	requireSequencerState(t, w, 7, prev)

	DeliverSequenceBatch(t, w, batch)
	requireSequencerState(t, w, 9, last)
}
