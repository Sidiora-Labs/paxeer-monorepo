package types_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestSequenceBatchTxRoundTrip(t *testing.T) {
	private, public := receiptKey()
	batch := batch31To42()
	batch.Height = 7
	batch = signedBatch(t, private, batch)
	tx, err := types.EncodeSequenceBatchTx(batch)
	require.NoError(t, err)
	require.Equal(t, 241, types.SequenceBatchTxLength)
	require.Len(t, tx, types.SequenceBatchTxLength)
	require.Equal(t, []byte(types.SequenceBatchDomain), tx[:25])
	require.Equal(t, mustHex(t, "0000000000000007000000000000001f000000000000002a"), tx[25:49])
	require.Equal(t, batch.PrevSeqHash, tx[49:81])
	require.Equal(t, batch.LastSeqHash, tx[81:113])
	require.Equal(t, batch.TxRoot, tx[113:145])
	require.Equal(t, batch.KeyId, tx[145:177])
	require.Equal(t, batch.Sig, tx[177:241])
	decoded, err := types.DecodeSequenceBatchTx(tx)
	require.NoError(t, err)
	require.Equal(t, batch, decoded)
	require.NoError(t, decoded.VerifySignature(testChainID, public))
	reencoded, err := types.EncodeSequenceBatchTx(decoded)
	require.NoError(t, err)
	require.Equal(t, tx, reencoded)

	t.Run("decoded fields own their bytes", func(t *testing.T) {
		input := bytes.Clone(tx)
		decoded, err := types.DecodeSequenceBatchTx(input)
		require.NoError(t, err)
		for i := range input {
			input[i] ^= 0xff
		}
		require.Equal(t, batch, decoded)
	})

	t.Run("full width integers", func(t *testing.T) {
		wide := batch
		wide.Height = 0x0102030405060708
		wide.FromSeq = 0x1112131415161718
		wide.ToSeq = 0xf1f2f3f4f5f6f7f8
		wide = signedBatch(t, private, wide)
		encoded, err := types.EncodeSequenceBatchTx(wide)
		require.NoError(t, err)
		require.Equal(t, mustHex(t, "01020304050607081112131415161718f1f2f3f4f5f6f7f8"), encoded[25:49])
		decoded, err := types.DecodeSequenceBatchTx(encoded)
		require.NoError(t, err)
		require.Equal(t, wide, decoded)
		require.NoError(t, decoded.VerifySignature(testChainID, public))
	})

	t.Run("empty batch", func(t *testing.T) {
		empty := batch
		empty.ToSeq = empty.FromSeq - 1
		empty.LastSeqHash = bytes.Clone(empty.PrevSeqHash)
		empty.TxRoot = bytes.Clone(types.EmptyTxRoot[:])
		empty = signedBatch(t, private, empty)
		encoded, err := types.EncodeSequenceBatchTx(empty)
		require.NoError(t, err)
		decoded, err := types.DecodeSequenceBatchTx(encoded)
		require.NoError(t, err)
		require.Equal(t, empty, decoded)
		require.True(t, decoded.IsEmpty())
		require.NoError(t, decoded.VerifySignature(testChainID, public))
	})

	for name, mutate := range map[string]func(*types.MsgSequenceBatch){
		"zero height":         func(m *types.MsgSequenceBatch) { m.Height = 0 },
		"zero fromSeq":        func(m *types.MsgSequenceBatch) { m.FromSeq = 0 },
		"invalid range":       func(m *types.MsgSequenceBatch) { m.ToSeq = m.FromSeq - 2 },
		"short prevSeqHash":   func(m *types.MsgSequenceBatch) { m.PrevSeqHash = m.PrevSeqHash[:31] },
		"long lastSeqHash":    func(m *types.MsgSequenceBatch) { m.LastSeqHash = append(bytes.Clone(m.LastSeqHash), 0) },
		"missing txRoot":      func(m *types.MsgSequenceBatch) { m.TxRoot = nil },
		"short keyId":         func(m *types.MsgSequenceBatch) { m.KeyId = m.KeyId[:31] },
		"short signature":     func(m *types.MsgSequenceBatch) { m.Sig = m.Sig[:63] },
		"empty hash mismatch": func(m *types.MsgSequenceBatch) { m.ToSeq = m.FromSeq - 1; m.TxRoot = types.EmptyTxRoot[:] },
		"empty root mismatch": func(m *types.MsgSequenceBatch) { m.ToSeq = m.FromSeq - 1; m.LastSeqHash = m.PrevSeqHash },
	} {
		t.Run("encode refuses "+name, func(t *testing.T) {
			invalid := batch
			mutate(&invalid)
			encoded, err := types.EncodeSequenceBatchTx(invalid)
			require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
			require.Nil(t, encoded)
		})
	}

	for name, mutate := range map[string]func([]byte){
		"zero height":   func(b []byte) { binary.BigEndian.PutUint64(b[25:33], 0) },
		"zero fromSeq":  func(b []byte) { binary.BigEndian.PutUint64(b[33:41], 0) },
		"invalid range": func(b []byte) { binary.BigEndian.PutUint64(b[41:49], batch.FromSeq-2) },
		"empty hash mismatch": func(b []byte) {
			binary.BigEndian.PutUint64(b[41:49], batch.FromSeq-1)
			copy(b[113:145], types.EmptyTxRoot[:])
		},
		"empty root mismatch": func(b []byte) {
			binary.BigEndian.PutUint64(b[41:49], batch.FromSeq-1)
			copy(b[81:113], batch.PrevSeqHash)
		},
	} {
		t.Run("decode refuses "+name, func(t *testing.T) {
			invalid := bytes.Clone(tx)
			mutate(invalid)
			decoded, err := types.DecodeSequenceBatchTx(invalid)
			require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
			require.Equal(t, types.MsgSequenceBatch{}, decoded)
		})
	}
}

func TestDecodeSequenceBatchTxRefusesLength(t *testing.T) {
	private, _ := receiptKey()
	tx, err := types.EncodeSequenceBatchTx(signedBatch(t, private, batch31To42()))
	require.NoError(t, err)
	for length := 0; length < types.SequenceBatchTxLength; length++ {
		t.Run(fmt.Sprintf("truncated to %d", length), func(t *testing.T) {
			decoded, err := types.DecodeSequenceBatchTx(tx[:length])
			require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
			require.Equal(t, types.MsgSequenceBatch{}, decoded)
		})
	}
	for _, input := range [][]byte{nil, append(bytes.Clone(tx), 0), append(bytes.Clone(tx), tx...)} {
		decoded, err := types.DecodeSequenceBatchTx(input)
		require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
		require.Equal(t, types.MsgSequenceBatch{}, decoded)
	}
}

func TestDecodeSequenceBatchTxRefusesDomain(t *testing.T) {
	private, _ := receiptKey()
	tx, err := types.EncodeSequenceBatchTx(signedBatch(t, private, batch31To42()))
	require.NoError(t, err)
	for i := range types.SequenceBatchDomain {
		t.Run(fmt.Sprintf("domain byte %d", i), func(t *testing.T) {
			invalid := bytes.Clone(tx)
			invalid[i] ^= 0xff
			require.Len(t, invalid, 241)
			decoded, err := types.DecodeSequenceBatchTx(invalid)
			require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
			require.Equal(t, types.MsgSequenceBatch{}, decoded)
		})
	}
}
