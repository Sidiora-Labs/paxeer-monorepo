// Proves: SeqHash/TxRoot + MsgSequenceBatch.SignBytes -> EncodeSequenceBatchTx + closed/reopened file -> DecodeSequenceBatchTx + MsgSequenceBatch.VerifySignature + VerifyBatchTxs.
//
// The sequencer-msg anchor names this test as the consumer of the exported Go
// surface. The producer derives every commitment from the input activities and
// signs it with a real ed25519 key; only its encoded output crosses the file.
// The file is synced and closed before consumption and repeated production must
// be byte-for-byte idempotent. These functions hold no service or store state.
// As in node/app.go's EncodeSequenceBatch and DecodeSequenceBatch wrappers, the
// crossing uses the types codec directly. Shape validation, authentication and
// activity verification remain separate checks, all exercised below.
package types_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

// produceProofSequenceBatch calls the contracted producers over real activity
// identifiers. It returns only the wire output; the consumer cannot reuse the
// producer's in-memory MsgSequenceBatch or its commitment buffers.
func produceProofSequenceBatch(t *testing.T, private ed25519.PrivateKey, height, from uint64, prev [32]byte, ids [][32]byte) []byte {
	t.Helper()
	last := prev
	for i, id := range ids {
		last = types.SeqHash(last, from+uint64(i), id)
	}
	root := types.TxRoot(ids)
	keyID := sha256.Sum256(private.Public().(ed25519.PublicKey))
	m := types.MsgSequenceBatch{
		Height: height, FromSeq: from, ToSeq: from - 1 + uint64(len(ids)),
		PrevSeqHash: prev[:], LastSeqHash: last[:], TxRoot: root[:], KeyId: keyID[:],
	}
	digest, err := m.SignBytes(testChainID)
	require.NoError(t, err)
	m.Sig = ed25519.Sign(private, digest[:])
	wire, err := types.EncodeSequenceBatchTx(m)
	require.NoError(t, err)
	return wire
}

func TestProofSequencerMsgEndToEnd(t *testing.T) {
	private, public := receiptKey()
	ids := [][32]byte{
		sha256.Sum256([]byte("first sequenced activity")),
		sha256.Sum256([]byte("second sequenced activity")),
		sha256.Sum256([]byte("third sequenced activity")),
	}
	for _, tc := range []struct {
		name string
		from uint64
		ids  [][32]byte
	}{
		{"ordered activities", 0x0102030405060708, ids},
		{"empty initial block", 1, nil},
		{"empty subsequent block", 256, [][32]byte{}},
		{"last uint64 sequence", math.MaxUint64 - 2, ids},
	} {
		t.Run(tc.name, func(t *testing.T) {
			height := uint64(0x1112131415161718)
			prev := sha256.Sum256([]byte("previous committed sequence hash"))
			path := filepath.Join(t.TempDir(), "tx0.bin")
			// End the producer step, including its file handle, before reading.
			func() {
				wire := produceProofSequenceBatch(t, private, height, tc.from, prev, tc.ids)
				file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
				require.NoError(t, err)
				n, writeErr := file.Write(wire)
				syncErr := file.Sync()
				closeErr := file.Close()
				require.NoError(t, writeErr)
				require.Equal(t, len(wire), n)
				require.NoError(t, syncErr)
				require.NoError(t, closeErr)
			}()
			file, err := os.Open(path)
			require.NoError(t, err)
			wire, readErr := io.ReadAll(file)
			closeErr := file.Close()
			require.NoError(t, readErr)
			require.NoError(t, closeErr)

			require.Equal(t, 241, types.SequenceBatchTxLength)
			require.Len(t, wire, types.SequenceBatchTxLength)
			require.Equal(t, "PAXEER/v1/sequence-batch\x00", string(wire[:25]))
			require.Equal(t, types.SequenceBatchDomain, string(wire[:25]))
			require.Equal(t, "PAXEER/v1/sequence\x00", types.SeqHashDomain)
			require.Equal(t, height, binary.BigEndian.Uint64(wire[25:33]))
			require.Equal(t, tc.from, binary.BigEndian.Uint64(wire[33:41]))
			to := tc.from - 1 + uint64(len(tc.ids))
			require.Equal(t, to, binary.BigEndian.Uint64(wire[41:49]))

			observed, err := types.DecodeSequenceBatchTx(wire)
			require.NoError(t, err)
			require.NoError(t, observed.VerifySignature(testChainID, public))
			require.NoError(t, types.VerifyBatchTxs(observed, tc.ids))
			require.Equal(t, height, observed.Height)
			require.Equal(t, tc.from, observed.FromSeq)
			require.Equal(t, to, observed.ToSeq)
			require.Equal(t, len(tc.ids) == 0, observed.IsEmpty())
			require.Equal(t, prev[:], observed.PrevSeqHash)
			keyID := sha256.Sum256(public[:])
			require.Equal(t, keyID[:], observed.KeyId)

			// Independent byte-level formulas catch a shared producer/verifier
			// error in domain, sequence endianness or activity order.
			var concatenated []byte
			last := prev
			for i, id := range tc.ids {
				concatenated = append(concatenated, id[:]...)
				preimage := append([]byte("PAXEER/v1/sequence\x00"), last[:]...)
				preimage = binary.BigEndian.AppendUint64(preimage, tc.from+uint64(i))
				preimage = append(preimage, id[:]...)
				last = sha256.Sum256(preimage)
			}
			root := sha256.Sum256(concatenated)
			require.Equal(t, root[:], observed.TxRoot)
			require.Equal(t, last[:], observed.LastSeqHash)
			require.Equal(t, observed.PrevSeqHash, wire[49:81])
			require.Equal(t, observed.LastSeqHash, wire[81:113])
			require.Equal(t, observed.TxRoot, wire[113:145])
			require.Equal(t, observed.KeyId, wire[145:177])
			require.Equal(t, observed.Sig, wire[177:241])

			reencoded, err := types.EncodeSequenceBatchTx(observed)
			require.NoError(t, err)
			require.Equal(t, wire, reencoded)
			repeated := produceProofSequenceBatch(t, private, height, tc.from, prev, tc.ids)
			require.Equal(t, wire, repeated, "repeated production is idempotent after reopen")
			replayed, err := types.DecodeSequenceBatchTx(repeated)
			require.NoError(t, err)
			require.NoError(t, replayed.VerifySignature(testChainID, public))
			require.NoError(t, types.VerifyBatchTxs(replayed, tc.ids))
			require.Equal(t, observed, replayed)

			// A transport buffer can be reused without changing consumer state.
			for i := range wire {
				wire[i] ^= 0xff
			}
			require.Equal(t, replayed, observed)
			require.NoError(t, observed.VerifySignature(testChainID, public))
			require.NoError(t, types.VerifyBatchTxs(observed, tc.ids))
		})
	}
}

func TestProofSequencerMsgRefusals(t *testing.T) {
	private, public := receiptKey()
	ids := [][32]byte{sha256.Sum256([]byte("activity one")), sha256.Sum256([]byte("activity two"))}
	wire := produceProofSequenceBatch(t, private, 123, 256, [32]byte{0x42}, ids)

	for n := 0; n < types.SequenceBatchTxLength; n++ {
		t.Run(fmt.Sprintf("truncated envelope %d", n), func(t *testing.T) {
			m, err := types.DecodeSequenceBatchTx(wire[:n])
			require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
			require.Equal(t, types.MsgSequenceBatch{}, m)
		})
	}
	for _, offset := range []int{0, 24} {
		invalid := bytes.Clone(wire)
		invalid[offset] ^= 1
		_, err := types.DecodeSequenceBatchTx(invalid)
		require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
	}
	_, err := types.DecodeSequenceBatchTx(append(bytes.Clone(wire), 0))
	require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)

	m, err := types.DecodeSequenceBatchTx(wire)
	require.NoError(t, err)
	otherPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x19}, ed25519.SeedSize))
	var otherPublic [32]byte
	copy(otherPublic[:], otherPrivate.Public().(ed25519.PublicKey))
	require.ErrorIs(t, m.VerifySignature(testChainID, otherPublic), types.ErrBadSignature)
	require.ErrorIs(t, m.VerifySignature(testChainID+"-other-chain", public), types.ErrBadSignature)

	tampered := bytes.Clone(wire)
	tampered[len(tampered)-1] ^= 1
	badSignature, err := types.DecodeSequenceBatchTx(tampered)
	require.NoError(t, err, "the codec checks shape; authentication is a consumer check")
	require.ErrorIs(t, badSignature.VerifySignature(testChainID, public), types.ErrBadSignature)

	require.ErrorIs(t, types.VerifyBatchTxs(m, ids[:1]), types.ErrInvalidSequenceBatch)
	swapped := [][32]byte{ids[1], ids[0]}
	err = types.VerifyBatchTxs(m, swapped)
	require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
	require.ErrorContains(t, err, "txRoot")

	// Even an authentic producer signing a changed root must be refused if
	// it leaves the chain bound to the original activity order.
	swappedRoot := types.TxRoot(swapped)
	m.TxRoot = swappedRoot[:]
	digest, err := m.SignBytes(testChainID)
	require.NoError(t, err)
	m.Sig = ed25519.Sign(private, digest[:])
	changedWire, err := types.EncodeSequenceBatchTx(m)
	require.NoError(t, err)
	changed, err := types.DecodeSequenceBatchTx(changedWire)
	require.NoError(t, err)
	require.NoError(t, changed.VerifySignature(testChainID, public))
	err = types.VerifyBatchTxs(changed, swapped)
	require.ErrorIs(t, err, types.ErrInvalidSequenceBatch)
	require.ErrorContains(t, err, "lastSeqHash")

	// Refusals leave the original producer output usable.
	original, err := types.DecodeSequenceBatchTx(wire)
	require.NoError(t, err)
	require.NoError(t, original.VerifySignature(testChainID, public))
	require.NoError(t, types.VerifyBatchTxs(original, ids))
}
