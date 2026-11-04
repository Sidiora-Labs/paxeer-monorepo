package sequencer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmutils "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func repairM1144Primary(t *testing.T, h *consensusFenceHarness, mode Mode) *Node {
	t.Helper()
	a := h.src.app
	a.ChainID = h.opts.ChainID
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	files := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(files.KeyFile()), 0700))
	pv := privval.NewFilePV(h.consensus.Key.PrivKey, files.KeyFile(), files.StateFile())
	require.NoError(t, pv.Save())
	cfg := DefaultConfig()
	cfg.Enable, cfg.Mode = true, mode
	cfg.WALDir = t.TempDir()
	cfg.ReceiptKeyFile = filepath.Join(t.TempDir(), "receipt-key")
	require.NoError(t, os.WriteFile(cfg.ReceiptKeyFile, []byte(hex.EncodeToString(h.src.key.private.Seed())), 0600))
	cfg.LeaseDuration = 2 * time.Second
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 4, 16
	cfg.MicroBatchWindow, cfg.MicroBatchMaxCount = 100*time.Millisecond, 4
	n, err := NewNode(cfg, a)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, n.Stop()) })
	return n
}

// Each API is a real transport endpoint with its own RPC feed. The shared-feed
// RPC lock is outside this task's scope and is recorded in repair_m1144.kvx.
func repairM1144API(t *testing.T, n *Node) *evmrpc.SendAPI {
	t.Helper()
	cfg := &evmrpc.SendConfig{Sequencer: n, SequencerSource: n.ReceiptSource}
	wm := evmrpc.NewWatermarkManager(nil, n.App.RPCContextProvider, nil, nil)
	wm.SetPendingSnapshot(n.Executor)
	return evmrpc.NewSendAPI(nil, func(int64) client.TxConfig { return n.App.GetTxConfig() }, cfg,
		&n.App.EvmKeeper, n.App.BeginBlockKeepers, n.App.RPCContextProvider, "", nil,
		n.App.BaseApp, n.App.TracerAnteHandler, evmrpc.ConnectionTypeHTTP,
		tmutils.None[time.Duration](), nil, nil, wm)
}

func repairM1144Signed(t *testing.T, n *Node, sender utils.TestAcct, nonce uint64) (*ethtypes.Transaction, []byte) {
	t.Helper()
	tx := consensusFenceTransfer(t, n.App, sender, nonce)
	decoded, err := n.App.GetTxConfig().TxDecoder()(tx)
	require.NoError(t, err)
	etx, _ := n.App.GetEVMMsg(decoded).AsTransaction()
	require.NotNil(t, etx)
	raw, err := etx.MarshalBinary()
	require.NoError(t, err)
	return etx, raw
}

func repairM1144Commit(t *testing.T, n *Node, h *consensusFenceHarness) tmtypes.SealedBlock {
	t.Helper()
	height := n.App.LastBlockHeight() + 1
	block, err := n.ConsensusSource.Seal(t.Context(), height)
	require.NoError(t, err)
	repair103Finalize(t, n.App, n.ConsensusSource, h.opts.ProposerAddress, height, block)
	return block
}

func TestRepairM_1144_MicrobatchLiveSerialization(t *testing.T) {
	RepairM1144LiveSerialization(t)
}

// RepairM1144LiveSerialization is shared with the public microbatch coverage.
func RepairM1144LiveSerialization(t *testing.T) {
	t.Helper()
	senders := []utils.TestAcct{utils.NewSigner(), utils.NewSigner(), utils.NewSigner(), utils.NewSigner()}
	h := newConsensusFenceHarness(t, 42, senders[0], func(ctx sdk.Context, a *app.App) {
		for _, s := range senders[1:] {
			a.EvmKeeper.SetAddressMapping(ctx, s.AccountAddress, s.EvmAddress)
			require.NoError(t, a.BankKeeper.SendCoins(ctx, senders[0].AccountAddress, s.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100_000_000_000))))
		}
	})
	n := repairM1144Primary(t, h, ModeMicroBatch)
	require.NoError(t, n.Start(t.Context()))
	type outcome struct {
		hash common.Hash
		err  error
	}
	out := make([]outcome, len(senders))
	raws := make([][]byte, len(senders))
	apis := make([]*evmrpc.SendAPI, len(senders))
	for i, s := range senders {
		_, raws[i] = repairM1144Signed(t, n, s, 0)
		apis[i] = repairM1144API(t, n)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range senders {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			out[i].hash, out[i].err = apis[i].SendRawTransaction(t.Context(), raws[i])
		}(i)
	}
	close(start)
	wg.Wait()
	type ordered struct {
		seq  uint64
		send *evmrpc.SequencerSend
		tx   []byte
	}
	order := make([]ordered, 0, len(senders))
	for i, o := range out {
		require.NoError(t, o.err)
		send, err := n.ReceiptSource.SequencerSend(o.hash)
		require.NoError(t, err)
		require.NotNil(t, send)
		verified, err := verify.ReceiptSignature(send.Receipt, n.ReceiptKey.PublicKey())
		require.NoError(t, err)
		require.Equal(t, senders[i].EvmAddress, common.BytesToAddress(verified.Receipt.From[:]))
		var tx []byte
		require.NoError(t, n.WAL.Scan(func(entry wal.Entry) error {
			if accepted, ok := entry.(*wal.Accept); ok && accepted.Seq == verified.Receipt.GlobalSequence {
				tx = append([]byte(nil), accepted.Tx...)
			}
			return nil
		}))
		require.NotEmpty(t, tx)
		order = append(order, ordered{verified.Receipt.GlobalSequence, send, tx})
	}
	mb := n.FeeFloor.Executor.(*microBatch)
	sizes := mb.GroupSizes()
	require.Contains(t, sizes, 4, "concurrent public producers must reach one real OCC group")
	sort.Slice(order, func(i, j int) bool { return order[i].seq < order[j].seq })
	// An independent sequential validator over the committed baseline must
	// reproduce every byte, including per-transaction balances and gas totals.
	opts := h.opts
	opts.Mode = ModeSequential
	exec, err := NewExecutor(n.App, opts)
	require.NoError(t, err)
	floor, err := NewFeeFloor(exec, n.App, "", n.App.ParamsKeeper, n.App.OracleKeeper, n.App.PricefeedKeeper, &n.App.EvmKeeper)
	require.NoError(t, err)
	r, err := NewReceipter(floor, n.App, &n.App.EvmKeeper, n.ReceiptKey, app.EVMReceiptModuleID)
	require.NoError(t, err)
	pending, err := n.Executor.Snapshot()
	require.NoError(t, err)
	require.NoError(t, floor.Open(2, pending.BlockTime().UnixMilli()))
	for i, o := range order {
		accepted, receipt, err := r.Accept(o.tx)
		require.NoError(t, err)
		require.Equal(t, uint64(43+i), accepted.Seq)
		require.Equal(t, o.send.Receipt, receipt, "group receipts must equal independent validator receipts")
		require.Equal(t, uint64(i+1)*21000, binary.BigEndian.Uint64(mustM1144Receipt(t, receipt).Effects[0].Body[8:16]))
	}
	block, err := n.ConsensusSource.Seal(t.Context(), 2)
	require.NoError(t, err)
	require.Len(t, block.Txs, 4)
	for i, o := range order {
		require.Equal(t, o.tx, []byte(block.Txs[i]))
	}
	repair103Finalize(t, n.App, n.ConsensusSource, h.opts.ProposerAddress, 2, block)
}

func mustM1144Receipt(t *testing.T, raw []byte) *codec.Receipt {
	t.Helper()
	decoded, err := codec.DecodeReceipt(raw)
	require.NoError(t, err)
	return decoded
}

func TestRepairM_1144_NonceSendAndOpenExpiry(t *testing.T) {
	sender := utils.NewSigner()
	n, h := repair103Node(t, sender, ModeSequential)
	require.NoError(t, n.Start(t.Context()))
	api := repairM1144API(t, n)
	future, raw := repairM1144Signed(t, n, sender, 1)
	hash, err := api.SendRawTransaction(t.Context(), raw)
	require.Equal(t, future.Hash(), hash)
	require.ErrorIs(t, err, ErrNonceHeld)
	require.True(t, n.NonceAdmission.queue.Held(sender.EvmAddress, 1))
	held, err := n.ReceiptSource.SequencerSend(hash)
	require.NoError(t, err)
	require.NotNil(t, held)
	require.Nil(t, held.Receipt)
	require.Equal(t, uint64(1), held.Nonce)
	require.Zero(t, n.App.EvmKeeper.GetNonce(n.App.LatestCtx(), sender.EvmAddress))
	current, raw := repairM1144Signed(t, n, sender, 0)
	_, err = api.SendRawTransaction(t.Context(), raw)
	require.NoError(t, err)
	require.Zero(t, n.NonceAdmission.queue.Len())
	for i, tx := range []*ethtypes.Transaction{current, future} {
		send, err := n.ReceiptSource.SequencerSend(tx.Hash())
		require.NoError(t, err)
		require.NotNil(t, send)
		decoded := mustM1144Receipt(t, send.Receipt)
		require.Equal(t, uint64(43+i), decoded.GlobalSequence)
		require.Equal(t, uint64(i), decoded.FromSequence)
	}
	require.Equal(t, uint64(2), n.App.EvmKeeper.GetNonce(n.App.LatestCtx(), sender.EvmAddress))
	expired, raw := repairM1144Signed(t, n, sender, 3)
	_, err = api.SendRawTransaction(t.Context(), raw)
	require.ErrorIs(t, err, ErrNonceHeld)
	q := n.NonceAdmission.queue
	q.mu.Lock()
	entry := q.held[sender.EvmAddress][3]
	entry.Arrived = time.Now().Add(-2 * n.Config.LeaseDuration)
	q.held[sender.EvmAddress][3] = entry
	q.mu.Unlock()
	repairM1144Commit(t, n, h)
	require.Zero(t, q.Len(), "new OPEN must expire held sends")
	lost, err := n.ReceiptSource.SequencerSend(expired.Hash())
	require.NoError(t, err)
	require.Nil(t, lost)
}

func TestRepairM_1144_NonceDrainPublicationFailure(t *testing.T) {
	sender := utils.NewSigner()
	n, _ := repair103Node(t, sender, ModeSequential)
	require.NoError(t, n.Start(t.Context()))
	api := repairM1144API(t, n)
	hashes := make([]common.Hash, 3)
	raws := make([][]byte, 3)
	for i := range raws {
		tx, raw := repairM1144Signed(t, n, sender, uint64(i))
		hashes[i], raws[i] = tx.Hash(), raw
	}
	for _, i := range []int{1, 2} {
		_, err := api.SendRawTransaction(t.Context(), raws[i])
		require.ErrorIs(t, err, ErrNonceHeld)
	}
	// Close the real WAL after nonce 0 is synced. Nonce 1 then executes but its
	// publication fails, so nonce 2 must stay held despite the advanced reader.
	n.Replicator.mu.Lock()
	n.Replicator.onWALSync = func(time.Duration) { n.WAL.Close() }
	n.Replicator.mu.Unlock()
	_, err := api.SendRawTransaction(t.Context(), raws[0])
	require.NoError(t, err)
	require.Equal(t, uint64(2), n.App.EvmKeeper.GetNonce(n.App.LatestCtx(), sender.EvmAddress))
	require.True(t, n.NonceAdmission.queue.Held(sender.EvmAddress, 2))
	require.Equal(t, 1, n.NonceAdmission.queue.Len())
	failures := n.NonceAdmission.queue.Failures()
	require.Len(t, failures, 1)
	require.Equal(t, uint64(1), failures[0].Nonce)
	require.ErrorIs(t, failures[0].Err, ErrReplicationHalted)
	send, err := n.ReceiptSource.SequencerSend(hashes[1])
	require.Nil(t, send)
	require.ErrorIs(t, err, ErrReplicationHalted)
	submitted, err := n.ReceiptSource.SequencerSend(hashes[2])
	require.NoError(t, err)
	require.NotNil(t, submitted)
	require.Nil(t, submitted.Receipt)
}

func TestRepairM_1144_ReceiptCapacityBeforeExecution(t *testing.T) {
	for _, mode := range []Mode{ModeSequential, ModeMicroBatch} {
		t.Run(mode.String(), func(t *testing.T) {
			sender := utils.NewSigner()
			n, h := repair103Node(t, sender, mode)
			n.ReceiptSource = NewReceiptSource(1)
			n.ReceiptSource.pending = n.pendingSend
			n.Accepter.src = n.ReceiptSource
			require.NoError(t, n.Start(t.Context()))
			api := repairM1144API(t, n)
			first, raw := repairM1144Signed(t, n, sender, 0)
			_, err := api.SendRawTransaction(t.Context(), raw)
			require.NoError(t, err)
			before, err := n.Executor.Snapshot()
			require.NoError(t, err)
			beforeBalance := n.App.EvmKeeper.GetBalance(before, sender.AccountAddress)
			tx, raw := repairM1144Signed(t, n, sender, 1)
			_, err = api.SendRawTransaction(t.Context(), raw)
			require.ErrorIs(t, err, ErrReceiptCapacity)
			after, err := n.Executor.Snapshot()
			require.NoError(t, err)
			require.Equal(t, uint64(1), n.App.EvmKeeper.GetNonce(after, sender.EvmAddress))
			require.Equal(t, 0, beforeBalance.Cmp(n.App.EvmKeeper.GetBalance(after, sender.AccountAddress)))
			require.Equal(t, uint64(43), baseSequential(n.FeeFloor.Executor).lastSeq)
			missing, err := n.ReceiptSource.SequencerSend(tx.Hash())
			require.NoError(t, err)
			require.Nil(t, missing)
			prior, err := n.ReceiptSource.SequencerSend(first.Hash())
			require.NoError(t, err)
			require.NotNil(t, prior)
			require.False(t, n.Accepter.halted)
			repairM1144Commit(t, n, h)
			_, err = api.SendRawTransaction(t.Context(), raw)
			require.NoError(t, err)
			kept, err := n.ReceiptSource.SequencerSend(tx.Hash())
			require.NoError(t, err)
			require.NotNil(t, kept)
			evicted, err := n.ReceiptSource.SequencerSend(first.Hash())
			require.NoError(t, err)
			require.Nil(t, evicted)
		})
	}
}

func TestRepairM_1144_LeaseLifecycleAndRestart(t *testing.T) {
	sender := utils.NewSigner()
	n, _ := repair103Node(t, sender, ModeSequential)
	n.Config.LeaseDuration = 120 * time.Millisecond
	before := time.Now()
	require.NoError(t, n.Start(t.Context()))
	holder := n.LeaseHolder
	require.NotNil(t, holder)
	initial := holder.Expiry()
	require.False(t, initial.Before(before.Add(n.Config.LeaseDuration).Truncate(time.Millisecond)))
	require.False(t, initial.After(time.Now().Add(n.Config.LeaseDuration)))
	require.Equal(t, uint64(1), holder.Epoch())
	require.Eventually(t, func() bool { return holder.Expiry().After(initial) }, time.Second, 5*time.Millisecond)
	// A failed renewal of the actual running primary must fence admission.
	n.WAL.Close()
	require.Eventually(t, func() bool { n.Accepter.mu.Lock(); defer n.Accepter.mu.Unlock(); return n.Accepter.halted }, time.Second, 5*time.Millisecond)
	_, err := repairM1144API(t, n).SendRawTransaction(t.Context(), func() []byte { _, raw := repairM1144Signed(t, n, sender, 0); return raw }())
	require.ErrorIs(t, err, ErrReplicationHalted)
	epoch := holder.Epoch()
	require.NoError(t, n.Stop())
	repair103ClearRuntime(n)
	reopened, err := NewNode(n.Config, n.App)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Stop()) })
	require.NoError(t, reopened.Start(t.Context()))
	require.Equal(t, epoch+1, reopened.LeaseHolder.Epoch())
	require.Equal(t, epoch+1, reopened.Replicator.Epoch())
	_, raw := repairM1144Signed(t, reopened, sender, 0)
	_, err = repairM1144API(t, reopened).SendRawTransaction(t.Context(), raw)
	require.NoError(t, err)
}

type repairM1144Pair struct {
	primary, standby *Node
	h                *consensusFenceHarness
	standbyID        [32]byte
}

func repairM1144Replicated(t *testing.T, sender utils.TestAcct) *repairM1144Pair {
	t.Helper()
	dir := t.TempDir()
	pv, err := privval.GenFilePV(filepath.Join(dir, "validator.json"), filepath.Join(dir, "state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	seed := bytes.Repeat([]byte{0x73}, ed25519.SeedSize)
	public := [32]byte(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))
	keyID := [32]byte{0x74}
	h := newConsensusFenceHarness(t, 42, sender, func(ctx sdk.Context, a *app.App) {
		keys, err := a.SequencerKeeper.ReceiptKeys(ctx)
		require.NoError(t, err)
		prior := keys[0]
		require.NoError(t, a.SequencerKeeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{
			{KeyId: prior.KeyId[:], PublicKey: prior.PublicKey[:], FirstHeight: prior.FirstHeight, LastHeight: prior.LastHeight},
			{KeyId: keyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 100},
		}))
		leaders, err := a.SequencerKeeper.Leaders(ctx)
		require.NoError(t, err)
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, append(leaders, [32]byte(pv.Key.PubKey.Bytes()))))
	})
	primary := repairM1144Primary(t, h, ModeSequential)
	backup := repair103Clone(t, primary.App)
	info, err := backup.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	files := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(files.KeyFile()), 0700))
	local := privval.NewFilePV(pv.Key.PrivKey, files.KeyFile(), files.StateFile())
	require.NoError(t, local.Save())
	primaryTLS, standbyTLS := Repair103TLSFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	cfg := primary.Config
	cfg.Role, cfg.StandbyAddress, cfg.ReplicationTLS = RoleStandby, address, standbyTLS
	cfg.WALDir = t.TempDir()
	cfg.ReceiptKeyFile = filepath.Join(t.TempDir(), "standby-receipt")
	require.NoError(t, os.WriteFile(cfg.ReceiptKeyFile, []byte(hex.EncodeToString(seed)), 0600))
	cfg.ReplicationTimeout = time.Second
	standby, err := NewNode(cfg, backup)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, standby.Stop()) })
	require.NoError(t, standby.Start(t.Context()))
	primary.Config.StandbyAddress, primary.Config.ReplicationTLS = address, primaryTLS
	primary.Config.ReplicationTimeout = time.Second
	primary.Replicator.Close()
	primary.Replicator, err = NewReplicator(ReplicatorConfig{
		WAL: primary.WAL, Dial: DialStandby(address, time.Second, primaryTLS), ReplicationTimeout: time.Second,
	})
	require.NoError(t, err)
	primary.Accepter.rep = primary.Replicator
	primary.ConsensusSource.repl = primary.Replicator
	primary.Driver.cfg.Replicator = primary.Replicator
	require.NoError(t, primary.Start(t.Context()))
	return &repairM1144Pair{primary: primary, standby: standby, h: h, standbyID: keyID}
}

func TestRepairM_1144_GovernedPinsCommitRefresh(t *testing.T) {
	sender := utils.NewSigner()
	p := repairM1144Replicated(t, sender)
	n, s := p.primary, p.standby
	api := repairM1144API(t, n)
	_, raw := repairM1144Signed(t, n, sender, 0)
	_, err := api.SendRawTransaction(t.Context(), raw)
	require.NoError(t, err)
	block, err := n.ConsensusSource.Seal(t.Context(), 2)
	require.NoError(t, err)
	keys, err := n.App.SequencerKeeper.ReceiptKeys(n.App.GetCheckCtx())
	require.NoError(t, err)
	old := keys[0]
	nextID := [32]byte{0x75}
	rotated := seqkeeper.ReceiptKeys{
		{KeyId: old.KeyId[:], PublicKey: old.PublicKey[:], FirstHeight: 1, LastHeight: 2},
		{KeyId: nextID[:], PublicKey: old.PublicKey[:], FirstHeight: 3, LastHeight: 100},
		{KeyId: keys[1].KeyId[:], PublicKey: keys[1].PublicKey[:], FirstHeight: 1, LastHeight: 100},
	}
	require.NoError(t, seqkeeper.ValidateRotation(seqkeeper.ReceiptKeys{
		{KeyId: old.KeyId[:], PublicKey: old.PublicKey[:], FirstHeight: 1, LastHeight: 100},
		{KeyId: keys[1].KeyId[:], PublicKey: keys[1].PublicKey[:], FirstHeight: 1, LastHeight: 100},
	}, rotated, 2))
	// Apply the governed keeper update in the actual block state, then commit
	// both validators before the primary's commit listener opens height 3.
	for _, a := range []*app.App{s.App, n.App} {
		response, err := a.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: a.ChainID, Height: 2, Time: block.OpenTime, ProposerAddress: p.h.opts.ProposerAddress},
			Txs:    append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...),
		})
		require.NoError(t, err)
		for _, result := range response.TxResults {
			require.Zero(t, result.Code, result.Log)
		}
		require.NoError(t, a.SequencerKeeper.SetReceiptKeys(a.GetContextForDeliverTx(nil), rotated))
		a.SetDeliverStateToCommit()
		_, err = a.Commit(t.Context())
		require.NoError(t, err)
	}
	require.Equal(t, n.App.LastCommitID().Hash, s.App.LastCommitID().Hash)
	id, err := n.ReceiptKey.KeyID(3)
	require.NoError(t, err)
	require.Equal(t, nextID, id)
	tx, raw := repairM1144Signed(t, n, sender, 1)
	_, err = api.SendRawTransaction(t.Context(), raw)
	require.NoError(t, err)
	send, err := n.ReceiptSource.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.NotNil(t, send)
	verified, err := verify.ReceiptSignature(send.Receipt, n.ReceiptKey.PublicKey())
	require.NoError(t, err)
	require.Equal(t, nextID, verified.Receipt.ContextHash)
	pin, err := rotated.KeyForHeight(nextID, 3)
	require.NoError(t, err)
	require.Equal(t, n.ReceiptKey.PublicKey(), [32]byte(pin.PublicKey))
	require.Eventually(t, func() bool { return s.Standby.LastSeq() == 44 }, time.Second, time.Millisecond)
	require.NoError(t, s.Standby.Err(), "standby must refresh committed primary authorization")
	replay := s.Standby.exec.(*standbyRoleReplay)
	require.Equal(t, uint64(3), replay.pins[1].FirstBatchNumber)
	_, err = n.ConsensusSource.Seal(t.Context(), 3)
	require.NoError(t, err)
}

func TestRepairM_1144_OperatorPromotionAndRecovery(t *testing.T) {
	sender := utils.NewSigner()
	p := repairM1144Replicated(t, sender)
	n, s := p.primary, p.standby
	oldKey := n.ReceiptKey.PublicKey()
	tx, raw := repairM1144Signed(t, n, sender, 0)
	_, err := repairM1144API(t, n).SendRawTransaction(t.Context(), raw)
	require.NoError(t, err)
	prior, err := n.ReceiptSource.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.NotNil(t, prior)
	originalEpoch := n.LeaseHolder.Epoch()
	require.NoError(t, n.Stop())
	require.NoError(t, s.Promote(t.Context()))
	require.Nil(t, s.Standby)
	require.Equal(t, RolePrimary, s.Config.Role)
	require.NotEqual(t, oldKey, s.ReceiptKey.PublicKey())
	require.Equal(t, p.standbyID, s.LeaseHolder.keyID)
	require.Greater(t, s.LeaseHolder.Epoch(), originalEpoch)
	require.Same(t, s.ConsensusSource, s.App.SequencerSource())
	recovered, err := s.ReceiptSource.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.Equal(t, prior, recovered)
	require.Equal(t, uint64(1), s.App.EvmKeeper.GetNonce(s.App.LatestCtx(), sender.EvmAddress))
	next, raw := repairM1144Signed(t, s, sender, 1)
	_, err = repairM1144API(t, s).SendRawTransaction(t.Context(), raw)
	require.NoError(t, err)
	send, err := s.ReceiptSource.SequencerSend(next.Hash())
	require.NoError(t, err)
	require.NotNil(t, send)
	verified, err := verify.ReceiptSignature(send.Receipt, s.ReceiptKey.PublicKey())
	require.NoError(t, err)
	require.Equal(t, p.standbyID, verified.Receipt.ContextHash)
	require.Equal(t, uint64(44), verified.Receipt.GlobalSequence)
	_, err = verify.ReceiptSignature(send.Receipt, oldKey)
	require.Error(t, err)
	epoch := s.LeaseHolder.Epoch()
	require.NoError(t, s.Stop())
	repair103ClearRuntime(s)
	reopened, err := NewNode(s.Config, s.App)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Stop()) })
	require.NoError(t, reopened.Start(t.Context()))
	require.Equal(t, epoch+1, reopened.LeaseHolder.Epoch())
	for hash, want := range map[common.Hash]*evmrpc.SequencerSend{tx.Hash(): prior, next.Hash(): send} {
		got, err := reopened.ReceiptSource.SequencerSend(hash)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	require.Equal(t, uint64(2), reopened.App.EvmKeeper.GetNonce(reopened.App.LatestCtx(), sender.EvmAddress))
	require.Error(t, reopened.Promote(t.Context()), "an existing primary cannot take over its own lease")
}

func TestRepairM_1144_LeaseExpiryFencesAdmission(t *testing.T) {
	sender := utils.NewSigner()
	n, _ := repair103Node(t, sender, ModeSequential)
	n.Config.LeaseDuration = 80 * time.Millisecond
	require.NoError(t, n.Start(t.Context()))
	// Hold the renewal writer past expiry; the public admission path must fence
	// from the configured clock even before the blocked renewer reports failure.
	n.Replicator.mu.Lock()
	locked := true
	defer func() {
		if locked {
			n.Replicator.mu.Unlock()
		}
	}()
	expiry := n.LeaseHolder.Expiry()
	timer := time.NewTimer(time.Until(expiry) + 10*time.Millisecond)
	<-timer.C
	_, raw := repairM1144Signed(t, n, sender, 0)
	_, err := repairM1144API(t, n).SendRawTransaction(t.Context(), raw)
	require.ErrorIs(t, err, ErrLeaseFenced)
	require.Zero(t, n.App.EvmKeeper.GetNonce(n.App.LatestCtx(), sender.EvmAddress))
	n.Replicator.mu.Unlock()
	locked = false
}

// Keep both sides of the public interface in the gate: JSON-RPC encodes a
// signed transaction, SendAPI reaches Node.Accept, and ReceiptSource returns it.
func TestRepairM_1144_SendTransport(t *testing.T) {
	sender := utils.NewSigner()
	n, _ := repair103Node(t, sender, ModeSequential)
	require.NoError(t, n.Start(t.Context()))
	api := repairM1144API(t, n)
	server := gethrpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", api))
	rpc := gethrpc.DialInProc(server)
	t.Cleanup(rpc.Close)
	tx, raw := repairM1144Signed(t, n, sender, 0)
	var hash common.Hash
	require.NoError(t, rpc.CallContext(context.Background(), &hash, "eth_sendRawTransaction", hexutilM1144(raw)))
	require.Equal(t, tx.Hash(), hash)
	send, err := n.ReceiptSource.SequencerSend(hash)
	require.NoError(t, err)
	require.NotNil(t, send)
	_, err = verify.ReceiptSignature(send.Receipt, n.ReceiptKey.PublicKey())
	require.NoError(t, err)
	var lease *wal.Lease
	require.NoError(t, n.WAL.Scan(func(entry wal.Entry) error {
		if value, ok := entry.(*wal.Lease); ok {
			lease = value
		}
		return nil
	}))
	require.NotNil(t, lease)
	require.Equal(t, n.LeaseHolder.Epoch(), lease.Epoch)
}

func hexutilM1144(raw []byte) string { return "0x" + hex.EncodeToString(raw) }
