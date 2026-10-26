package app_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/stretchr/testify/require"
)

func TestCheckTxRefusesSequenceBatchEncoding(t *testing.T) {
	a := app.Setup(t, false, false, false)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyID := sha256.Sum256(pub)
	batch := seqtypes.MsgSequenceBatch{
		Height:      1,
		FromSeq:     1,
		ToSeq:       0,
		PrevSeqHash: make([]byte, seqtypes.HashLength),
		LastSeqHash: make([]byte, seqtypes.HashLength),
		TxRoot:      seqtypes.EmptyTxRoot[:],
		KeyId:       keyID[:],
	}
	digest, err := batch.SignBytes("pax-test")
	require.NoError(t, err)
	batch.Sig = ed25519.Sign(priv, digest[:])
	encoded, err := app.EncodeSequenceBatch(batch)
	require.NoError(t, err)

	for _, mode := range []struct {
		name   string
		typeID abci.CheckTxTypeV2
	}{
		{"new", abci.CheckTxTypeV2New},
		{"recheck", abci.CheckTxTypeV2Recheck},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				tx   []byte
			}{
				{"encoded", encoded},
				{"domain_only", []byte(seqtypes.SequenceBatchDomain)},
				{"truncated", encoded[:len(encoded)-1]},
			} {
				t.Run(tc.name, func(t *testing.T) {
					res := a.CheckTx(context.Background(), &abci.RequestCheckTxV2{
						Tx: tc.tx, Type: mode.typeID,
					})
					require.NotNil(t, res)
					require.NotNil(t, res.ResponseCheckTx)
					require.Equal(t, uint32(8), res.Code)
					require.Equal(t, seqtypes.ModuleName, res.Codespace)
					require.Contains(t, res.Log, seqkeeper.ErrBatchBlockOnly.Error())
				})
			}
		})
	}
}
