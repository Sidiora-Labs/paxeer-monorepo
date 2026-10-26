package types_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func batchForVerifyTxs(from uint64, ids [][32]byte) types.MsgSequenceBatch {
	prev := [32]byte{0x42}
	last := prev
	for i, id := range ids {
		last = types.SeqHash(last, from+uint64(i), id)
	}
	root := types.TxRoot(ids)
	return types.MsgSequenceBatch{
		Height:      10,
		FromSeq:     from,
		ToSeq:       from - 1 + uint64(len(ids)),
		PrevSeqHash: prev[:],
		LastSeqHash: last[:],
		TxRoot:      root[:],
	}
}

func TestVerifyBatchTxsAcceptsChain(t *testing.T) {
	ids := [][32]byte{{0x11}, {0x22}, {0x33}}
	batch := batchForVerifyTxs(5, ids)
	require.NoError(t, types.VerifyBatchTxs(batch, ids))

	t.Run("empty", func(t *testing.T) {
		empty := batchForVerifyTxs(8, nil)
		require.True(t, empty.IsEmpty())
		require.Equal(t, types.EmptyTxRoot[:], empty.TxRoot)
		require.NoError(t, types.VerifyBatchTxs(empty, nil))
		require.NoError(t, types.VerifyBatchTxs(empty, [][32]byte{}))

		wrongRoot := empty
		wrongRoot.TxRoot = make([]byte, types.HashLength)
		require.ErrorIs(t, types.VerifyBatchTxs(wrongRoot, nil), types.ErrInvalidSequenceBatch)

		wrongLast := empty
		wrongLast.LastSeqHash = make([]byte, types.HashLength)
		require.ErrorIs(t, types.VerifyBatchTxs(wrongLast, nil), types.ErrInvalidSequenceBatch)
	})

	t.Run("maximum sequence", func(t *testing.T) {
		last := batchForVerifyTxs(math.MaxUint64-2, ids)
		require.Equal(t, uint64(math.MaxUint64), last.ToSeq)
		require.NoError(t, types.VerifyBatchTxs(last, ids))
	})

	for name, mutate := range map[string]func(*types.MsgSequenceBatch){
		"wrong root":               func(m *types.MsgSequenceBatch) { m.TxRoot = make([]byte, types.HashLength) },
		"wrong last hash":          func(m *types.MsgSequenceBatch) { m.LastSeqHash = make([]byte, types.HashLength) },
		"wrong previous hash":      func(m *types.MsgSequenceBatch) { m.PrevSeqHash = make([]byte, types.HashLength) },
		"short previous hash":      func(m *types.MsgSequenceBatch) { m.PrevSeqHash = m.PrevSeqHash[:31] },
		"long previous hash":       func(m *types.MsgSequenceBatch) { m.PrevSeqHash = append(bytes.Clone(m.PrevSeqHash), 0) },
		"wrong sequence positions": func(m *types.MsgSequenceBatch) { m.FromSeq++; m.ToSeq++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := batch
			mutate(&changed)
			require.ErrorIs(t, types.VerifyBatchTxs(changed, ids), types.ErrInvalidSequenceBatch)
		})
	}
}

func TestVerifyBatchTxsRefusesSwappedTxs(t *testing.T) {
	ids := [][32]byte{{0x11}, {0x22}, {0x33}}
	batch := batchForVerifyTxs(5, ids)
	swapped := [][32]byte{ids[1], ids[0], ids[2]}
	err := types.VerifyBatchTxs(batch, swapped)
	require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
	require.ErrorContains(t, err, "txRoot")

	swappedRoot := types.TxRoot(swapped)
	batch.TxRoot = swappedRoot[:]
	err = types.VerifyBatchTxs(batch, swapped)
	require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
	require.ErrorContains(t, err, "lastSeqHash")
}

func TestVerifyBatchTxsRefusesCountMismatch(t *testing.T) {
	ids := [][32]byte{{0x11}, {0x22}, {0x33}}
	batch := batchForVerifyTxs(5, ids)
	for name, supplied := range map[string][][32]byte{
		"too few":  ids[:2],
		"too many": {ids[0], ids[1], ids[2], {0x44}},
		"none":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			err := types.VerifyBatchTxs(batch, supplied)
			require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
			require.ErrorContains(t, err, "count")
			invalidHashes := batch
			invalidHashes.TxRoot = nil
			invalidHashes.PrevSeqHash = nil
			invalidHashes.LastSeqHash = nil
			err = types.VerifyBatchTxs(invalidHashes, supplied)
			require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
			require.ErrorContains(t, err, "count")
		})
	}

	empty := batchForVerifyTxs(8, nil)
	err := types.VerifyBatchTxs(empty, ids[:1])
	require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
	require.ErrorContains(t, err, "count")
}
