package sequencer_test

import (
	"math/big"
	"testing"
	"time"

	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestExecutorSeqHashMatchesModuleSeqHash(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode sequencer.Mode
	}{
		{"sequential", sequencer.ModeSequential},
		{"microbatch", sequencer.ModeMicroBatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const initialSeq = uint64(0x01020304050607fe)
			sender := utils.NewSigner()
			recipient := utils.NewSigner().EvmAddress
			h := newHarness(t, sender)
			exec, err := sequencer.NewExecutor(h.app, sequencer.Options{
				Mode:            tc.mode,
				ChainID:         testChainID,
				ProposerAddress: h.proposer,
				LastSeq:         initialSeq,
				LastSeqHash:     startSeqHash,
				MicroBatch: sequencer.MicroBatchOptions{
					Window: time.Millisecond, MaxCount: 4,
				},
			})
			require.NoError(t, err)
			h.exec = exec
			ts := h.open()

			wantHash := startSeqHash
			var txs [][]byte
			for nonce := uint64(0); nonce < 3; nonce++ {
				signed, bz := transfer(t, sender, nonce, recipient, big.NewInt(1))
				accepted, err := exec.Accept(bz)
				require.NoError(t, err)
				require.NotNil(t, accepted)
				seq := initialSeq + nonce + 1
				require.Equal(t, seq, accepted.Seq)
				require.Equal(t, wantHash, accepted.PrevSeqHash)
				require.Equal(t, [32]byte(signed.Hash()), accepted.ActivityID)
				wantHash = seqtypes.SeqHash(wantHash, seq, accepted.ActivityID)
				require.Equal(t, wantHash, accepted.SeqHash)
				require.Equal(t, int(nonce)+1, accepted.TxIndex)
				txs = append(txs, bz)

				refused, err := exec.Accept(bz)
				requireRefused(t, err, sdkerrors.ErrWrongSequence)
				require.Nil(t, refused)
			}

			batch, err := exec.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, openHeight, batch.Height)
			require.Equal(t, ts, batch.TimestampMs)
			require.Equal(t, txs, batch.Txs)
			require.Len(t, batch.Results, len(txs))
			require.Equal(t, initialSeq+1, batch.FromSeq)
			require.Equal(t, initialSeq+uint64(len(txs)), batch.LastSeq)
			require.Equal(t, startSeqHash, batch.PrevSeqHash)
			require.Equal(t, wantHash, batch.LastSeqHash)
		})
	}
}
