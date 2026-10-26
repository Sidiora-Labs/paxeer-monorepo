package sequencer_test

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestMicroBatchPlacesTxWithSetTxPosition(t *testing.T) {
	const groupSize = 4
	senders := make([]utils.TestAcct, groupSize)
	for i := range senders {
		senders[i] = utils.NewSigner()
	}
	recipient := utils.NewSigner().EvmAddress
	h := newHarness(t, senders...)
	counting := &microbatchCountingApp{App: h.app}
	mb, err := sequencer.NewMicroBatch(counting, microbatchOptions(h), sequencer.MicroBatchOptions{
		Window: 50 * time.Millisecond, MaxCount: groupSize,
	})
	require.NoError(t, err)
	ts := h.genesis.Add(time.Second).UnixMilli()
	require.NoError(t, mb.Open(openHeight, ts))

	type transaction struct {
		evm *ethtypes.Transaction
		bz  []byte
		acc *sequencer.Accepted
		err error
	}
	ordered := make([]*transaction, 2*groupSize)
	for wave := 0; wave < 2; wave++ {
		group := make([]*transaction, groupSize)
		for i, sender := range senders {
			etx, bz := transfer(t, sender, uint64(wave), recipient, big.NewInt(onePaxInWei))
			group[i] = &transaction{evm: etx, bz: bz}
		}
		if wave == 1 {
			etx, bz := transfer(t, senders[0], 9, recipient, big.NewInt(1))
			group = append(group, &transaction{evm: etx, bz: bz})
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, tx := range group {
			wg.Add(1)
			go func(tx *transaction) {
				defer wg.Done()
				<-start
				tx.acc, tx.err = mb.Accept(tx.bz)
			}(tx)
		}
		close(start)
		wg.Wait()
		for i, tx := range group {
			if i == groupSize {
				require.Nil(t, tx.acc)
				requireRefused(t, tx.err, sdkerrors.ErrWrongSequence)
				continue
			}
			require.NoError(t, tx.err)
			require.NotNil(t, tx.acc)
			require.Zero(t, tx.acc.Result.Code, tx.acc.Result.Log)
			require.Greater(t, tx.acc.TxIndex, wave*groupSize)
			require.LessOrEqual(t, tx.acc.TxIndex, (wave+1)*groupSize)
			require.Nil(t, ordered[tx.acc.TxIndex-1])
			ordered[tx.acc.TxIndex-1] = tx
		}
	}
	batch, err := mb.Seal(openHeight)
	require.NoError(t, err)
	require.Len(t, batch.Txs, len(ordered))
	counting.mu.Lock()
	sizes := append([]int(nil), counting.sizes...)
	counting.mu.Unlock()
	largest := 0
	for _, size := range sizes {
		largest = max(largest, size)
	}
	require.Greater(t, largest, 1, "must exercise delivery of multiple entries together")

	require.NoError(t, h.exec.Open(openHeight, ts))
	for i, tx := range ordered {
		require.NotNil(t, tx)
		acc, err := h.exec.Accept(tx.bz)
		require.NoError(t, err)
		require.Equal(t, i+1, acc.TxIndex)
		require.Equal(t, acc, tx.acc)
		sequentialReceipt, err := h.app.EvmKeeper.GetTransientReceipt(h.openCtx(), tx.evm.Hash(), uint64(i+1))
		require.NoError(t, err)
		batchReceipt, err := h.app.EvmKeeper.GetTransientReceipt(batch.Ctx, tx.evm.Hash(), uint64(i+1))
		require.NoError(t, err)
		require.Equal(t, uint32(1), batchReceipt.Status)
		require.Equal(t, uint32(i+1), batchReceipt.TransactionIndex)
		require.Equal(t, sequentialReceipt, batchReceipt)
	}
	sequentialBatch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, sequentialBatch.Txs, batch.Txs)
	require.Equal(t, sequentialBatch.Results, batch.Results)
	require.Equal(t, sequentialBatch.LastSeqHash, batch.LastSeqHash)
}
