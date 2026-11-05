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
	in.PaymentDenom = "upax"
	in.Accepted.Height = height
	in.Accepted.Seq += uint64(height) * 2
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
	require.Equal(t, &send, got)
	got, err = source.SequencerSend(nextHash)
	require.NoError(t, err)
	require.Equal(t, &next, got)

	source.Record(lastHash, last)
	got, err = source.SequencerSend(hash)
	require.NoError(t, err)
	require.Nil(t, got, "a full source evicts its oldest committed send to admit a pending one")
	for hash, want := range map[common.Hash]evmrpc.SequencerSend{nextHash: next, lastHash: last} {
		got, err := source.SequencerSend(hash)
		require.NoError(t, err)
		require.Equal(t, &want, got)
	}
	source.Committed(vectorHeight - 1)
	source.Record(hash, send)
	got, err = source.SequencerSend(hash)
	require.NoError(t, err)
	require.Nil(t, got, "an evicted duplicate cannot restore the send")
	source.Committed(vectorHeight + 3)
	for hash, want := range map[common.Hash]evmrpc.SequencerSend{nextHash: next, lastHash: last} {
		got, err := source.SequencerSend(hash)
		require.NoError(t, err)
		require.Equal(t, &want, got)
	}

	var zero sequencer.ReceiptSource
	zero.Record(nextHash, next)
	got, err = zero.SequencerSend(nextHash)
	require.NoError(t, err)
	require.Equal(t, &next, got)
	zero.Committed(vectorHeight + 1)
	got, err = zero.SequencerSend(nextHash)
	require.NoError(t, err)
	require.Equal(t, &next, got)
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

func TestReceiptSourceServesCommittedSend(t *testing.T) {
	hash, send := receiptSourceAcceptedSend(t, 7)
	source := sequencer.NewReceiptSource(2)
	source.Record(hash, send)
	for _, height := range []int64{7, 9} {
		source.Committed(height)
		got, err := source.SequencerSend(hash)
		require.NoError(t, err)
		require.Equal(t, &send, got)
	}
}

func TestReceiptSourceEvictsOldestCommittedSendAtCapacity(t *testing.T) {
	firstHash, first := receiptSourceAcceptedSend(t, 1)
	secondHash, second := receiptSourceAcceptedSend(t, 2)
	thirdHash, third := receiptSourceAcceptedSend(t, 3)
	fourthHash, fourth := receiptSourceAcceptedSend(t, 4)

	assertSend := func(source *sequencer.ReceiptSource, hash common.Hash, want *evmrpc.SequencerSend) {
		t.Helper()
		got, err := source.SequencerSend(hash)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	source := sequencer.NewReceiptSource(2)
	source.Record(firstHash, first)
	source.Record(secondHash, second)
	source.Committed(2)
	source.Record(thirdHash, third)
	assertSend(source, firstHash, nil)
	assertSend(source, secondHash, &second)
	assertSend(source, thirdHash, &third)
	source.Record(fourthHash, fourth)
	assertSend(source, firstHash, nil)
	assertSend(source, secondHash, nil)
	assertSend(source, thirdHash, &third)
	assertSend(source, fourthHash, &fourth)
	require.PanicsWithValue(t, "sequencer: receipt source capacity exhausted before commit", func() {
		source.Record(firstHash, first)
		// A late record is ignored even when no committed send remains to evict.
		source.Record(secondHash, second)
		nextHash, next := receiptSourceAcceptedSend(t, 5)
		source.Record(nextHash, next)
	})
	assertSend(source, thirdHash, &third)
	assertSend(source, fourthHash, &fourth)

	uncommitted := sequencer.NewReceiptSource(2)
	uncommitted.Record(firstHash, first)
	uncommitted.Record(secondHash, second)
	require.PanicsWithValue(t, "sequencer: receipt source capacity exhausted before commit", func() {
		uncommitted.Record(thirdHash, third)
	})
	assertSend(uncommitted, firstHash, &first)
	assertSend(uncommitted, secondHash, &second)
	assertSend(uncommitted, thirdHash, nil)

	t.Run("height order precedes insertion order", func(t *testing.T) {
		source := sequencer.NewReceiptSource(2)
		source.Record(secondHash, second)
		source.Record(firstHash, first)
		source.Committed(2)
		source.Record(thirdHash, third)
		got, err := source.SequencerSend(firstHash)
		require.NoError(t, err)
		require.Nil(t, got)
		got, err = source.SequencerSend(secondHash)
		require.NoError(t, err)
		require.Equal(t, &second, got)
	})

	t.Run("insertion order breaks height ties", func(t *testing.T) {
		in := vectorInput()
		in.PaymentDenom = "upax"
		in.Accepted.Height = 1
		in.Accepted.Seq += 3
		in.Accepted.ActivityID[0] ^= 0x80
		in.Accepted.SeqHash = expectedSeqHash(in.Accepted.PrevSeqHash, in.Accepted.Seq, in.Accepted.ActivityID)
		in.EVM.Status = 0
		in.EVM.VmError = "execution reverted"
		receipt, err := vectorKey(t).SignReceipt(in)
		require.NoError(t, err)
		sameHeightHash := common.Hash(in.Accepted.ActivityID)
		sameHeight := first
		sameHeight.Receipt = receipt
		source := sequencer.NewReceiptSource(2)
		source.Record(firstHash, first)
		source.Record(sameHeightHash, sameHeight)
		source.Record(firstHash, first)
		source.Committed(1)
		source.Committed(2)
		source.Record(thirdHash, third)
		got, err := source.SequencerSend(firstHash)
		require.NoError(t, err)
		require.Nil(t, got)
		got, err = source.SequencerSend(sameHeightHash)
		require.NoError(t, err)
		require.Equal(t, &sameHeight, got)
		source.Record(fourthHash, fourth)
		got, err = source.SequencerSend(sameHeightHash)
		require.NoError(t, err)
		require.Nil(t, got)
		got, err = source.SequencerSend(thirdHash)
		require.NoError(t, err)
		require.Equal(t, &third, got)
	})
}
