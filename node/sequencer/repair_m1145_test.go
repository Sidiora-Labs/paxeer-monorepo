package sequencer

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1145_ConcurrentFirstPublication(t *testing.T) {
	seed := sha256.Sum256([]byte("repair m1145 first publication"))
	private := ed25519.NewKeyFromSeed(seed[:])
	key := &ReceiptKey{receiptKeyState: &receiptKeyState{
		private: private, public: [32]byte(private.Public().(ed25519.PublicKey)),
	}}
	oldID, nextID := [32]byte{1}, [32]byte{2}
	pins := []verify.SequencerAuthorization{
		{SequencerID: oldID, PublicKey: key.PublicKey(), FirstBatchNumber: 1, LastBatchNumber: 4},
		{SequencerID: nextID, PublicKey: key.PublicKey(), FirstBatchNumber: 5, LastBatchNumber: 9},
	}
	_, err := key.KeyID(1)
	require.ErrorIs(t, err, ErrReceiptKeyNotPinned)
	start := make(chan struct{})
	results := make(chan error, 8)
	for worker := range 8 {
		// Node startup and seal-retry fixtures copy this handle. Every copy must
		// retain the same identity and pin slot, including before the first store.
		copyKey := *key
		go func() {
			<-start
			for iteration := range 1000 {
				owned := append([]verify.SequencerAuthorization(nil), pins...)
				if (worker+iteration)%2 == 0 {
					owned[0], owned[1] = owned[1], owned[0]
				}
				if err := copyKey.SetPins(owned); err != nil {
					results <- err
					return
				}
				// The caller retains its input; publication must own a deep value copy.
				owned[0].SequencerID = [32]byte{99}
				for height := int64(1); height <= 9; height++ {
					want := oldID
					if height >= 5 {
						want = nextID
					}
					id, err := copyKey.KeyID(height)
					if err != nil || id != want {
						results <- fmt.Errorf("height %d: id %x, error %v", height, id, err)
						return
					}
				}
				for _, height := range []int64{-1, 0, 10} {
					if _, err := copyKey.KeyID(height); !errors.Is(err, ErrReceiptKeyNotPinned) {
						results <- fmt.Errorf("height %d accepted outside pin bounds: %v", height, err)
						return
					}
				}
			}
			results <- nil
		}()
	}
	close(start)
	for range 8 {
		require.NoError(t, <-results)
	}
	for _, height := range []int64{4, 5} {
		id, err := key.KeyID(height)
		require.NoError(t, err)
		want := oldID
		if height == 5 {
			want = nextID
		}
		require.Equal(t, want, id, "copies must publish to the original handle")
	}
}

func repairM1145Refresh(t *testing.T, key *ReceiptKey, pins []verify.SequencerAuthorization) (*atomic.Uint64, func()) {
	t.Helper()
	var updates atomic.Uint64
	stop := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan error, 2)
	for worker := range 2 {
		go func() {
			announced := false
			for {
				select {
				case <-stop:
					results <- nil
					return
				default:
				}
				owned := append([]verify.SequencerAuthorization(nil), pins...)
				if worker == 1 {
					for i, j := 0, len(owned)-1; i < j; i, j = i+1, j-1 {
						owned[i], owned[j] = owned[j], owned[i]
					}
				}
				if err := key.SetPins(owned); err != nil {
					results <- err
					ready <- struct{}{}
					return
				}
				owned[0].SequencerID = [32]byte{0xff}
				invalid := append([]verify.SequencerAuthorization(nil), pins...)
				invalid[len(invalid)-1].SequencerID = [32]byte{}
				if err := key.SetPins(invalid); !errors.Is(err, ErrReceiptKeyPin) {
					results <- fmt.Errorf("invalid refresh was not refused: %v", err)
					ready <- struct{}{}
					return
				}
				updates.Add(1)
				if !announced {
					ready <- struct{}{}
					announced = true
				}
				runtime.Gosched()
			}
		}()
	}
	var once sync.Once
	finish := func() {
		once.Do(func() {
			close(stop)
			for range 2 {
				require.NoError(t, <-results)
			}
		})
	}
	t.Cleanup(finish)
	for range 2 {
		<-ready
	}
	return &updates, finish
}

func TestRepairM_1145_RefreshReceiptAndConsensusSeal(t *testing.T) {
	sender := utils.NewSigner()
	pair := repairM1144Replicated(t, sender)
	n, standby := pair.primary, pair.standby
	server := gethrpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", repairM1144API(t, n)))
	client := gethrpc.DialInProc(server)
	t.Cleanup(client.Close)
	keys, err := n.App.SequencerKeeper.ReceiptKeys(n.App.GetCheckCtx())
	require.NoError(t, err)
	old := keys[0]
	nextID := [32]byte{0x45}
	rotated := seqkeeper.ReceiptKeys{
		{KeyId: old.KeyId[:], PublicKey: old.PublicKey[:], FirstHeight: 1, LastHeight: 2},
		{KeyId: nextID[:], PublicKey: old.PublicKey[:], FirstHeight: 3, LastHeight: 5},
		{KeyId: keys[1].KeyId[:], PublicKey: keys[1].PublicKey[:], FirstHeight: 1, LastHeight: 100},
	}
	require.NoError(t, seqkeeper.ValidateRotation(seqkeeper.ReceiptKeys{
		{KeyId: old.KeyId[:], PublicKey: old.PublicKey[:], FirstHeight: 1, LastHeight: 100},
		{KeyId: keys[1].KeyId[:], PublicKey: keys[1].PublicKey[:], FirstHeight: 1, LastHeight: 100},
	}, rotated, 2))
	for height := int64(2); height <= 3; height++ {
		storedKeys, err := n.App.SequencerKeeper.ReceiptKeys(n.App.GetCheckCtx())
		require.NoError(t, err)
		committedKeys := make(seqkeeper.ReceiptKeys, 0, len(storedKeys))
		for _, stored := range storedKeys {
			committedKeys = append(committedKeys, seqkeeper.ReceiptKeyEntry{
				KeyId: stored.KeyId[:], PublicKey: stored.PublicKey[:],
				FirstHeight: stored.FirstHeight, LastHeight: stored.LastHeight,
			})
		}
		pins, err := n.App.SequencerKeeper.ReceiptKeyPins(n.App.GetCheckCtx())
		require.NoError(t, err)
		updates, stop := repairM1145Refresh(t, n.ReceiptKey, pins)
		before := updates.Load()
		wantID := old.KeyId
		if height == 3 {
			wantID = nextID
		}
		var receipts [][]byte
		for offset := uint64(0); offset < 4; offset++ {
			nonce := uint64(height-2)*4 + offset
			tx, raw := repairM1144Signed(t, n, sender, nonce)
			var hash common.Hash
			require.NoError(t, client.CallContext(t.Context(), &hash, "eth_sendRawTransaction", hexutilM1144(raw)))
			require.Equal(t, tx.Hash(), hash)
			send, err := n.ReceiptSource.SequencerSend(hash)
			require.NoError(t, err)
			require.NotNil(t, send)
			verified, err := verify.ReceiptSignature(send.Receipt, n.ReceiptKey.PublicKey())
			require.NoError(t, err)
			_, err = committedKeys.VerifyReceipt(send.Receipt)
			require.NoError(t, err)
			require.Equal(t, wantID, verified.Receipt.ContextHash)
			require.Equal(t, uint64(height), binary.BigEndian.Uint64(verified.Receipt.BatchID[24:]))
			require.Equal(t, [32]byte(hash), verified.Receipt.ActivityID)
			require.Equal(t, uint64(43)+nonce, verified.Receipt.GlobalSequence)
			receipts = append(receipts, send.Receipt)
		}
		require.Greater(t, updates.Load(), before, "pin publication must progress while the real send path signs")
		before = updates.Load()
		block, err := n.ConsensusSource.Seal(t.Context(), height)
		require.NoError(t, err)
		require.Greater(t, updates.Load(), before, "pin publication must progress while consensus seals")
		stop()
		require.Len(t, block.Txs, 4)
		msg, err := app.DecodeSequenceBatch(block.Batch)
		require.NoError(t, err)
		require.Equal(t, wantID[:], msg.KeyId)
		require.Equal(t, uint64(height), msg.Height)
		pin, err := committedKeys.KeyForHeight(wantID, uint64(height))
		require.NoError(t, err)
		require.NoError(t, msg.VerifySignature(n.App.ChainID, [32]byte(pin.PublicKey)))
		seen := 0
		require.NoError(t, n.WAL.Scan(func(entry wal.Entry) error {
			if accepted, ok := entry.(*wal.Accept); ok && accepted.Height == uint64(height) {
				require.Less(t, seen, len(receipts))
				require.Equal(t, receipts[seen], accepted.Receipt)
				require.Equal(t, []byte(block.Txs[seen]), accepted.Tx)
				seen++
			}
			return nil
		}))
		require.Equal(t, 4, seen)
		for _, a := range []*app.App{standby.App, n.App} {
			response, err := a.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
				Header: &tmproto.Header{ChainID: a.ChainID, Height: height, Time: block.OpenTime, ProposerAddress: pair.h.opts.ProposerAddress},
				Txs:    append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...),
			})
			require.NoError(t, err)
			require.Len(t, response.TxResults, 5)
			for _, result := range response.TxResults {
				require.Zero(t, result.Code, result.Log)
			}
			if height == 2 {
				require.NoError(t, a.SequencerKeeper.SetReceiptKeys(a.GetContextForDeliverTx(nil), rotated))
			}
			a.SetDeliverStateToCommit()
			_, err = a.Commit(t.Context())
			require.NoError(t, err)
		}
		require.Equal(t, n.App.LastCommitID().Hash, standby.App.LastCommitID().Hash)
		require.NoError(t, standby.Standby.Err())
	}
	for _, height := range []int64{-1, 0, 6, 100} {
		_, err := n.ReceiptKey.KeyID(height)
		require.ErrorIs(t, err, ErrReceiptKeyNotPinned)
	}
	id, err := n.ReceiptKey.KeyID(2)
	require.NoError(t, err)
	require.Equal(t, old.KeyId, id)
	id, err = n.ReceiptKey.KeyID(3)
	require.NoError(t, err)
	require.Equal(t, nextID, id)
}
