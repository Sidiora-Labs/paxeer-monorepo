package sequencer

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestReceiptSourceReplayRestoresAcceptedSends(t *testing.T) {
	sender := utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender)
	dir := filepath.Join(t.TempDir(), "wal")
	log, err := wal.OpenWAL(dir, wal.Config{SegmentSizeLimit: 1})
	require.NoError(t, err)
	t.Cleanup(log.Close)
	rep, err := NewReplicator(ReplicatorConfig{WAL: log, Standalone: true})
	require.NoError(t, err)
	t.Cleanup(rep.Close)
	r, err := NewReceipter(h.src.exec, h.src.app, &h.src.app.EvmKeeper, h.src.key, 12)
	require.NoError(t, err)
	metrics, err := NewMetrics(otel.Meter("receipt-source-replay"))
	require.NoError(t, err)
	live := NewReceiptSource(3)
	accepter := NewAccepter(r, rep, live, metrics)
	require.NoError(t, h.src.exec.Open(2, h.openedAt))
	_, err = rep.Release(t.Context(), &wal.Open{Height: 2, OpenTimeMs: uint64(h.openedAt), BaseFee: uint256.NewInt(0)})
	require.NoError(t, err)
	hashes := make([]common.Hash, 3)
	sends := make([]*evmrpc.SequencerSend, 3)
	txs := make([][]byte, 3)
	for i := range hashes {
		txs[i] = consensusFenceTransfer(t, h.src.app, sender, uint64(i))
		accepted, err := accepter.Accept(txs[i])
		require.NoError(t, err)
		decoded, err := codec.DecodeReceipt(accepted.Receipt)
		require.NoError(t, err)
		require.Zero(t, decoded.ResultCode)
		hashes[i] = common.Hash(decoded.ActivityID)
		sends[i], err = live.SequencerSend(hashes[i])
		require.NoError(t, err)
		require.NotNil(t, sends[i])
		require.Equal(t, accepted.Receipt, sends[i].Receipt)
	}
	batch, err := h.src.exec.Seal(2)
	require.NoError(t, err)
	_, err = rep.Release(t.Context(), &wal.Seal{Height: 2, FromSeq: batch.FromSeq, ToSeq: batch.LastSeq, LastSeqHash: batch.LastSeqHash})
	require.NoError(t, err)
	rep.Close()
	log.Close()
	reopened, err := wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(reopened.Close)

	for _, tc := range []struct {
		name      string
		capacity  int
		committed int64
		retained  int
	}{
		{"all sends survive seal", 3, 1, 3},
		{"newest sends win before replayed seal", 2, 1, 2},
		{"newest sends win below existing watermark", 2, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec, err := NewExecutor(h.src.app, h.opts)
			require.NoError(t, err)
			receipter, err := NewReceipter(exec, h.src.app, &h.src.app.EvmKeeper, h.src.key, 12)
			require.NoError(t, err)
			source := NewReceiptSource(tc.capacity)
			source.Committed(tc.committed)
			replayer := &nodeStartReplayer{AppReplayer: NewAppReplayer(exec, receipter), ctx: t.Context()}
			replayer.src = source
			result, err := reopened.Replay(replayer)
			require.NoError(t, err)
			require.Equal(t, len(hashes), result.Accepts)
			require.True(t, replayer.sealed)
			require.Equal(t, int64(2), source.committed)
			require.Len(t, source.entries, tc.retained)
			require.Empty(t, source.pendingOrder)
			for i, hash := range hashes {
				got, err := source.SequencerSend(hash)
				require.NoError(t, err)
				if i < len(hashes)-tc.retained {
					require.Nil(t, got)
				} else {
					require.Equal(t, sends[i], got)
				}
			}
			require.Equal(t, hashes[len(hashes)-tc.retained:], source.committedOrder)
			source.Record(hashes[0], *sends[0])
			got, err := source.SequencerSend(hashes[0])
			require.NoError(t, err)
			if tc.retained < len(hashes) {
				require.Nil(t, got, "live records at committed heights must not restore evicted sends")
			} else {
				require.Equal(t, sends[0], got)
			}
			require.ErrorIs(t, replayer.Apply(&wal.Seal{Height: 3}), ErrLayerNotOpen)
			require.Equal(t, int64(2), source.committed, "failed seals must not advance the watermark")
		})
	}

	t.Run("mismatched receipt is never recorded", func(t *testing.T) {
		exec, err := NewExecutor(h.src.app, h.opts)
		require.NoError(t, err)
		receipter, err := NewReceipter(exec, h.src.app, &h.src.app.EvmKeeper, h.src.key, 12)
		require.NoError(t, err)
		source := NewReceiptSource(3)
		replayer := NewAppReplayer(exec, receipter)
		replayer.src = source
		require.NoError(t, replayer.Apply(&wal.Open{Height: 2, OpenTimeMs: uint64(h.openedAt), BaseFee: uint256.NewInt(0)}))
		tampered := bytes.Clone(sends[0].Receipt)
		tampered[len(tampered)-1] ^= 1
		got, err := replayer.Execute(&wal.Accept{Seq: h.opts.LastSeq + 1, Height: 2, Tx: txs[0], Receipt: tampered})
		require.ErrorIs(t, err, wal.ErrReceiptMismatch)
		require.Nil(t, got)
		stored, err := source.SequencerSend(hashes[0])
		require.NoError(t, err)
		require.Nil(t, stored)
		require.Empty(t, source.entries)
	})

	_, err = reopened.Append(&wal.Committed{Height: 2, LastSeq: batch.LastSeq})
	require.NoError(t, err)
	_, err = reopened.GroupSync()
	require.NoError(t, err)
	t.Run("committed sends are scanned without executing", func(t *testing.T) {
		for _, capacity := range []int{3, 2} {
			source := NewReceiptSource(capacity)
			require.NoError(t, source.restoreCommitted(t.Context(), reopened))
			require.Equal(t, int64(2), source.committed)
			require.Empty(t, source.pendingOrder)
			require.Equal(t, hashes[len(hashes)-capacity:], source.committedOrder)
			for i, hash := range hashes {
				got, err := source.SequencerSend(hash)
				require.NoError(t, err)
				if i < len(hashes)-capacity {
					require.Nil(t, got)
				} else {
					require.Equal(t, sends[i], got)
				}
			}
		}
		exec, err := NewExecutor(h.src.app, h.opts)
		require.NoError(t, err)
		receipter, err := NewReceipter(exec, h.src.app, &h.src.app.EvmKeeper, h.src.key, 12)
		require.NoError(t, err)
		result, err := reopened.Replay(NewAppReplayer(exec, receipter))
		require.NoError(t, err)
		require.Zero(t, result.Accepts)
		require.Nil(t, result.LastAccept)
		_, err = exec.Snapshot()
		require.ErrorIs(t, err, ErrLayerNotOpen)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, NewReceiptSource(3).restoreCommitted(ctx, reopened), context.Canceled)
		stopped := errors.New("stop scan")
		require.ErrorIs(t, reopened.Scan(func(wal.Entry) error { return stopped }), stopped)
	})
}
