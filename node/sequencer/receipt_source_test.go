package sequencer_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func receiptSourceAcceptedSend(t *testing.T, height int64) (common.Hash, evmrpc.SequencerSend) {
	t.Helper()
	in := vectorInput()
	in.Accepted.Height = height
	in.Accepted.ActivityID[0] ^= byte(height)
	in.Accepted.SeqHash = expectedSeqHash(in.Accepted.PrevSeqHash, in.Accepted.Seq, in.Accepted.ActivityID)
	in.EVM.Status = 0
	in.EVM.VmError = "execution reverted"
	receipt, err := vectorKey(t).SignReceipt(in)
	require.NoError(t, err)
	logs := make([][]byte, len(in.EVM.Logs))
	for i, log := range in.EVM.Logs {
		logs[i] = bytes.Clone(log.Data)
	}
	return common.Hash(in.Accepted.ActivityID), evmrpc.SequencerSend{
		Nonce: in.FromSequence, Receipt: receipt, ReturnData: in.ReturnData,
		LogData: logs, VMError: in.EVM.VmError,
	}
}

func TestReceiptSourceRecordsAcceptedSend(t *testing.T) {
	hash, send := receiptSourceAcceptedSend(t, vectorHeight)
	nextHash, next := receiptSourceAcceptedSend(t, vectorHeight+1)
	lastHash, last := receiptSourceAcceptedSend(t, vectorHeight+2)
	source := sequencer.NewReceiptSource(2)
	source.Record(hash, send)
	source.Record(nextHash, next)
	got, err := source.SequencerSend(hash)
	require.NoError(t, err)
	require.Equal(t, &send, got)

	t.Run("owns recorded and returned bytes", func(t *testing.T) {
		wantReceipt := bytes.Clone(send.Receipt)
		wantReturn := bytes.Clone(send.ReturnData)
		wantLog := bytes.Clone(send.LogData[0])
		send.Receipt[0] ^= 0xff
		send.ReturnData[0] ^= 0xff
		send.LogData[0][0] ^= 0xff
		got.Receipt[0] ^= 0x7f
		got.ReturnData[0] ^= 0x7f
		got.LogData[0][0] ^= 0x7f
		got.LogData[1] = []byte("changed empty log")
		again, err := source.SequencerSend(hash)
		require.NoError(t, err)
		require.NotNil(t, again)
		require.Equal(t, wantReceipt, again.Receipt)
		require.Equal(t, wantReturn, again.ReturnData)
		require.Equal(t, wantLog, again.LogData[0])
		require.Nil(t, again.LogData[1])
		send = *again
	})

	t.Run("capacity preserves uncommitted receipts", func(t *testing.T) {
		source.Record(nextHash, next)
		require.Panics(t, func() { source.Record(lastHash, last) })
		for hash, want := range map[common.Hash]evmrpc.SequencerSend{hash: send, nextHash: next} {
			got, err := source.SequencerSend(hash)
			require.NoError(t, err)
			require.Equal(t, &want, got)
		}
	})

	source.Committed(vectorHeight - 1)
	got, err = source.SequencerSend(hash)
	require.NoError(t, err)
	require.Equal(t, &send, got)
	source.Committed(vectorHeight)
	got, err = source.SequencerSend(hash)
	require.NoError(t, err)
	require.Nil(t, got)
	got, err = source.SequencerSend(nextHash)
	require.NoError(t, err)
	require.Equal(t, &next, got)

	source.Record(lastHash, last)
	source.Committed(vectorHeight - 1)
	source.Record(hash, send)
	got, err = source.SequencerSend(hash)
	require.NoError(t, err)
	require.Nil(t, got, "a late record must not restore a committed receipt")
	source.Committed(vectorHeight + 3)
	for _, hash := range []common.Hash{nextHash, lastHash} {
		got, err := source.SequencerSend(hash)
		require.NoError(t, err)
		require.Nil(t, got)
	}

	var zero sequencer.ReceiptSource
	zero.Record(nextHash, next)
	got, err = zero.SequencerSend(nextHash)
	require.NoError(t, err)
	require.Equal(t, &next, got)
	zero.Committed(vectorHeight + 1)
	got, err = zero.SequencerSend(nextHash)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestReceiptSourceUnknownHashNil(t *testing.T) {
	var source sequencer.ReceiptSource
	unknown := common.HexToHash("0xdeadbeef")
	got, err := source.SequencerSend(unknown)
	require.NoError(t, err)
	require.Nil(t, got)
	hash, send := receiptSourceAcceptedSend(t, vectorHeight)
	source.Record(hash, send)
	got, err = source.SequencerSend(unknown)
	require.NoError(t, err)
	require.Nil(t, got)
	source.Committed(vectorHeight)
	got, err = source.SequencerSend(unknown)
	require.NoError(t, err)
	require.Nil(t, got)
}
