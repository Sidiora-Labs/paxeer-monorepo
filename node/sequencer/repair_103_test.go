package sequencer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	protoio "github.com/gogo/protobuf/io"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair103Transfer(t *testing.T, a *app.App, sender utils.TestAcct, chainID int64, nonce uint64) []byte {
	t.Helper()
	to := common.HexToAddress("0xde")
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: nonce, GasPrice: big.NewInt(500_000_000_000), Gas: 21000, To: &to, Value: big.NewInt(1)}), ethtypes.LatestSignerForChainID(big.NewInt(chainID)), sender.EvmPrivateKey)
	require.NoError(t, err)
	data, err := ethtx.NewTxDataFromTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	b := a.GetTxConfig().NewTxBuilder()
	require.NoError(t, b.SetMsgs(msg))
	b.SetGasLimit(21000)
	raw, err := a.GetTxConfig().TxEncoder()(b.GetTx())
	require.NoError(t, err)
	return raw
}

func repair103Node(t *testing.T, sender utils.TestAcct, mode Mode) (*Node, *consensusFenceHarness) {
	t.Helper()
	h := newConsensusFenceHarness(t, 42, sender)
	a := h.src.app
	a.ChainID = h.opts.ChainID
	h.opts.ProposerAddress = h.consensus.Key.PubKey.Address()
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	paths := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.KeyFile()), 0700))
	pv := privval.NewFilePV(h.consensus.Key.PrivKey, paths.KeyFile(), paths.StateFile())
	require.NoError(t, pv.Save())
	cfg := DefaultConfig()
	cfg.Enable, cfg.Mode = true, mode
	cfg.WALDir = filepath.Join(t.TempDir(), "wal")
	cfg.ReceiptKeyFile = filepath.Join(t.TempDir(), "receipt-key")
	require.NoError(t, os.WriteFile(cfg.ReceiptKeyFile, []byte(hex.EncodeToString(h.src.key.private.Seed())), 0600))
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 4, 8
	cfg.LeaseDuration = time.Second
	cfg.MicroBatchWindow, cfg.MicroBatchMaxCount = time.Millisecond, 4
	n, err := NewNode(cfg, a)
	require.NoError(t, err)
	t.Cleanup(func() { _ = n.Stop() })
	return n, h
}

func repair103ClearRuntime(n *Node) {
	n.App.SetOpenBlock(nil)
	n.App.SetSequencerSource(nil)
	n.App.SetSequencerRPC(evmrpc.SequencerOption{}, evmrpc.SequencerReceiptOption{})
	n.App.SetCommitListener(nil)
}

func repair103Finalize(t *testing.T, a *app.App, source tmtypes.SequencerSource, proposer []byte, height int64, block tmtypes.SealedBlock) {
	t.Helper()
	a.SetSequencerSource(source)
	response, err := a.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: a.ChainID, Height: height, Time: block.OpenTime, ProposerAddress: proposer},
		Txs:    append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...),
	})
	require.NoError(t, err)
	for _, result := range response.TxResults {
		require.Zero(t, result.Code, result.Log)
	}
	a.SetDeliverStateToCommit()
	_, err = a.Commit(t.Context())
	require.NoError(t, err)
	require.Equal(t, height, a.LastBlockHeight())
}

func repair103Clone(t *testing.T, a *app.App) *app.App {
	t.Helper()
	var snapshot bytes.Buffer
	require.NoError(t, a.CommitMultiStore().Snapshot(uint64(a.LastBlockHeight()), protoio.NewDelimitedWriter(&snapshot)))
	// Use the production state-commitment store for this replica. Historical
	// query indexing is optional and is not part of replication or recovery.
	opts := viper.New()
	opts.Set("chain-id", a.ChainID)
	opts.Set(app.FlagSCEnable, true)
	opts.Set(app.FlagSCAsyncCommitBuffer, 0)
	opts.Set(app.FlagSCSnapshotInterval, 0)
	opts.Set(app.FlagSSEnable, false)
	opts.Set("evm.http_enabled", false)
	opts.Set("evm.ws_enabled", false)
	clone := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, false,
		tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, opts, app.EmptyWasmOpts, nil,
		func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(false)
			ba.SetStoreLoader(func(ms sdk.CommitMultiStore) error {
				if err := ms.LoadLatestVersion(); err != nil {
					return err
				}
				_, err := ms.Restore(uint64(a.LastBlockHeight()), 0, protoio.NewDelimitedReader(bytes.NewReader(snapshot.Bytes()), 64<<20))
				return err
			})
		})
	clone.ChainID = a.ChainID
	t.Cleanup(func() { require.NoError(t, clone.Close()) })
	require.Equal(t, a.LastCommitID().Hash, clone.LastCommitID().Hash)
	return clone
}

func TestRepair_103_MicrobatchAdmission(t *testing.T) {
	for _, limit := range []string{"gas", "wanted", "bytes", "domain-gas", "domain-bytes"} {
		t.Run(limit, func(t *testing.T) {
			previous, existed := evmconfig.ChainIDMapping["pax-test"]
			evmconfig.ChainIDMapping["pax-test"] = 125
			t.Cleanup(func() {
				if existed {
					evmconfig.ChainIDMapping["pax-test"] = previous
				} else {
					delete(evmconfig.ChainIDMapping, "pax-test")
				}
			})
			first, second, gap := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
			var txs [][]byte
			chainID := int64(125)
			if limit == "domain-gas" || limit == "domain-bytes" {
				chainID = chainstypes.ChainIDOf(7).Int64()
			}
			h := newConsensusFenceHarness(t, 42, first, func(ctx sdk.Context, a *app.App) {
				for _, sender := range []utils.TestAcct{second, gap} {
					a.EvmKeeper.SetAddressMapping(ctx, sender.AccountAddress, sender.EvmAddress)
					require.NoError(t, a.BankKeeper.SendCoins(ctx, first.AccountAddress, sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100_000_000_000))))
				}
				// A nonce-gap candidate precedes both valid candidates. Its failed
				// delivery must not consume any block or tenant reservation.
				txs = [][]byte{repair103Transfer(t, a, gap, chainID, 2), repair103Transfer(t, a, first, chainID, 0), repair103Transfer(t, a, second, chainID, 0)}
				// Admission precedes execution: even the nonce-gap candidate must
				// fit individually before delivery can refuse its nonce. Its
				// protobuf nonce field can make it larger than a nonce-zero send.
				candidateBytes := max(len(txs[0]), len(txs[1]), len(txs[2]))
				cp := *a.GetConsensusParams(ctx)
				block := *cp.Block
				block.MaxGas, block.MaxGasWanted, block.MaxBytes = 1_000_000, 1_000_000, 200_000
				switch limit {
				case "gas":
					block.MaxGas = 21000
				case "wanted":
					block.MaxGasWanted = 21000
				case "bytes":
					tx0, err := app.EncodeSequenceBatch(seqtypes.MsgSequenceBatch{Height: 1, FromSeq: 1, ToSeq: 1, PrevSeqHash: make([]byte, 32), LastSeqHash: make([]byte, 32), TxRoot: make([]byte, 32), KeyId: make([]byte, 32), Sig: make([]byte, 64)})
					require.NoError(t, err)
					block.MaxBytes = int64(len(tx0) + candidateBytes)
				}
				cp.Block = &block
				a.StoreConsensusParams(ctx, &cp)
				if chainID != 125 {
					genesis := chainstypes.DefaultGenesis()
					genesis.Params.MinGasAllowance, genesis.Params.MinByteAllowance = 2, 1
					gas, size := uint64(1_000_000), uint64(200_000)
					if limit == "domain-gas" {
						gas = 21000
					} else {
						size = uint64(candidateBytes)
					}
					genesis.Domains = append(genesis.Domains, chainstypes.Domain{Id: 7, Name: "repair", Status: chainstypes.StatusActive, GasAllowance: gas, ByteAllowance: size, TargetGasPerBlock: gas / 2, Bond: sdk.ZeroInt()})
					genesis.NextBaseFees[7], genesis.NextID = sdk.ZeroDec(), 8
					require.NoError(t, a.LayerXChainsKeeper.InitGenesis(ctx, *genesis))
				}
			})
			opts := h.opts
			opts.Mode = ModeMicroBatch
			exec, err := NewMicroBatch(h.src.app, opts, MicroBatchOptions{Window: time.Second, MaxCount: 3})
			require.NoError(t, err)
			require.NoError(t, exec.Open(2, h.openedAt))
			// Execute the real group runner used by Accept in a fixed arrival
			// order; the real OCC scheduler supplies every execution result.
			m := exec.(*microBatch)
			out := m.execute(txs)
			require.Nil(t, out[0].acc)
			require.Error(t, out[0].err)
			require.NotErrorIs(t, out[0].err, ErrBlockFull)
			require.NoError(t, out[1].err)
			require.NotNil(t, out[1].acc)
			require.Equal(t, uint64(43), out[1].acc.Seq)
			require.Nil(t, out[2].acc)
			require.ErrorIs(t, out[2].err, ErrBlockFull)
			usage, ok := m.open.ctx.Context().Value(admissionKey{}).(admissionUsage)
			require.True(t, ok)
			require.Equal(t, uint64(21000), usage.gas)
			require.Equal(t, int64(len(txs[1])), usage.bytes, "count only the accepted candidate, once")
			if chainID != 125 {
				require.Equal(t, chainAdmissionUsage{gas: 21000, bytes: uint64(len(txs[1]))}, usage.chains[7])
			}
			receipter, err := NewReceipter(exec, h.src.app, &h.src.app.EvmKeeper, h.src.key, app.EVMReceiptModuleID)
			require.NoError(t, err)
			refused, receipt, err := receipter.Accept(txs[2])
			require.ErrorIs(t, err, ErrBlockFull)
			require.Nil(t, refused)
			require.Nil(t, receipt, "an over-limit send cannot receive a signed receipt")
			source, err := NewConsensusSource(h.src.app, exec, h.src.key)
			require.NoError(t, err)
			block, err := source.Seal(t.Context(), 2)
			require.NoError(t, err)
			require.Equal(t, tmtypes.Txs{txs[1]}, block.Txs)
			// The validator independently enforces the same block and domain limits.
			txData := append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...)
			digest := sha256.Sum256(bytes.Join(txData, nil))
			response, err := h.src.app.ProcessProposal(t.Context(), &abci.RequestProcessProposal{Hash: digest[:], Header: &tmproto.Header{ChainID: h.opts.ChainID, Height: 2, Time: block.OpenTime, ProposerAddress: h.opts.ProposerAddress}, Txs: txData})
			require.NoError(t, err)
			require.Equal(t, abci.ResponseProcessProposal_ACCEPT, response.Status)
			if info := h.src.app.GetOptimisticProcessingInfo(); info.Completion != nil {
				select {
				case <-info.Completion:
				case <-time.After(time.Second):
					t.Fatal("validator processing did not finish")
				}
			}
			h.src.app.ClearOptimisticProcessingInfo()
		})
	}
}

func TestRepair_103_SealedTailRestart(t *testing.T) {
	for _, mode := range []Mode{ModeSequential, ModeMicroBatch} {
		for _, committedBeforeCrash := range []bool{false, true} {
			name := mode.String()
			if committedBeforeCrash {
				name += "/commit-before-WAL"
			}
			t.Run(name, func(t *testing.T) {
				sender := utils.NewSigner()
				n, h := repair103Node(t, sender, mode)
				require.NoError(t, n.Start(t.Context()))
				sends := make(map[common.Hash][]byte)
				var txs tmtypes.Txs
				for nonce := uint64(0); nonce < 3; nonce++ {
					tx := consensusFenceTransfer(t, n.App, sender, nonce)
					accepted, err := n.Accepter.Accept(tx)
					require.NoError(t, err)
					decoded, err := codec.DecodeReceipt(accepted.Receipt)
					require.NoError(t, err)
					sends[common.Hash(decoded.ActivityID)] = accepted.Receipt
					txs = append(txs, tx)
				}
				block, err := n.ConsensusSource.Seal(t.Context(), 2)
				require.NoError(t, err)
				if committedBeforeCrash {
					n.App.SetCommitListener(nil) // crash point: application commit precedes Driver.COMMITTED
					repair103Finalize(t, n.App, n.ConsensusSource, h.opts.ProposerAddress, 2, block)
				}
				require.NoError(t, n.Stop())
				repair103ClearRuntime(n)
				reopened, err := NewNode(n.Config, n.App)
				require.NoError(t, err)
				t.Cleanup(func() { _ = reopened.Stop() })
				require.NoError(t, reopened.Start(t.Context()))
				for hash, receipt := range sends {
					send, err := reopened.ReceiptSource.SequencerSend(hash)
					require.NoError(t, err)
					require.NotNil(t, send)
					require.Equal(t, receipt, send.Receipt)
					_, err = verify.ReceiptSignature(send.Receipt, reopened.ReceiptKey.PublicKey())
					require.NoError(t, err)
				}
				if !committedBeforeCrash {
					recovered, err := reopened.ConsensusSource.Seal(t.Context(), 2)
					require.NoError(t, err)
					require.Equal(t, block, recovered)
					require.Equal(t, txs, recovered.Txs)
					repair103Finalize(t, reopened.App, reopened.ConsensusSource, h.opts.ProposerAddress, 2, recovered)
				}
				require.Equal(t, uint64(45), reopened.WAL.LastCommitted().LastSeq)
				require.Equal(t, int64(2), reopened.App.LastBlockHeight())
				require.Equal(t, uint64(3), reopened.App.EvmKeeper.GetNonce(reopened.App.GetCheckCtx(), sender.EvmAddress))
			})
		}
	}
}

// repair103Pair uses the production executor, receipt builder, codec, WAL and
// authenticated TCP link. Both apps start from the same committed snapshot.
type repair103Pair struct {
	n      *Node
	h      *consensusFenceHarness
	backup *app.App
	sb     *Standby
	source *standbyRoleSource
	replay *standbyRoleReplay
	cfg    StandbyConfig
	dir    string
	stop   func()
}

func repair103Replicated(t *testing.T, sender utils.TestAcct) *repair103Pair {
	t.Helper()
	n, h := repair103Node(t, sender, ModeSequential)
	backup := repair103Clone(t, n.App)
	// The replica verifies primary receipts; its own key is distinct.
	secondary := newConsensusFenceHarness(t, 42, sender)
	exec, err := NewExecutor(backup, h.opts)
	require.NoError(t, err)
	r, err := NewReceipter(exec, backup, &backup.EvmKeeper, n.ReceiptKey, app.EVMReceiptModuleID)
	require.NoError(t, err)
	pins, err := n.App.SequencerKeeper.ReceiptKeyPins(n.App.GetCheckCtx())
	require.NoError(t, err)
	source := &standbyRoleSource{app: backup}
	replay := &standbyRoleReplay{AppReplayer: NewAppReplayer(exec, r), pins: pins, local: secondary.src.key.public, source: source}
	dir := t.TempDir()
	w, err := wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	public, err := h.consensus.GetPubKey(t.Context())
	require.NoError(t, err)
	cfg := StandbyConfig{WAL: w, Executor: replay, ConsensusKey: secondary.consensus, ReceiptKey: secondary.src.key.private, PrimaryConsensusKey: public, PrimaryReceiptKey: n.ReceiptKey.private.Public().(ed25519.PublicKey), CommittedHeight: 1, LastSeq: 42, ReplicationTimeout: time.Second}
	ctx, cancel := context.WithCancel(t.Context())
	sb, err := NewStandby(ctx, cfg)
	require.NoError(t, err)
	listener, auth := repair103TLSListener(t)
	done := make(chan error, 1)
	go func() { done <- sb.Serve(standbyRoleListener{Listener: listener, ctx: ctx, timeout: time.Second}) }()
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); _ = listener.Close(); <-done; w.Close() }) }
	t.Cleanup(stop)
	n.Replicator.Close()
	n.Replicator, err = NewReplicator(ReplicatorConfig{WAL: n.WAL, Dial: DialStandby(listener.Addr().String(), time.Second, auth), ReplicationTimeout: time.Second})
	require.NoError(t, err)
	n.Accepter.rep, n.ConsensusSource.repl, n.Driver.cfg.Replicator = n.Replicator, n.Replicator, n.Replicator
	backup.SetOpenBlock(exec)
	backup.SetSequencerSource(source)
	require.NoError(t, n.Start(t.Context()))
	return &repair103Pair{n: n, h: h, backup: backup, sb: sb, source: source, replay: replay, cfg: cfg, dir: dir, stop: stop}
}

func TestRepair_103_SealWaitsForAcceptWAL(t *testing.T) {
	sender := utils.NewSigner()
	p := repair103Replicated(t, sender)
	n := p.n
	// Stall the real WAL release after execution has entered L_open.
	n.Replicator.mu.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			n.Replicator.mu.Unlock()
		}
	}()
	accepted := make(chan struct {
		send evmrpc.AcceptedSend
		err  error
	}, 1)
	tx := consensusFenceTransfer(t, n.App, sender, 0)
	go func() {
		send, err := n.Accepter.Accept(tx)
		accepted <- struct {
			send evmrpc.AcceptedSend
			err  error
		}{send, err}
	}()
	require.Eventually(t, func() bool {
		ctx, err := n.Executor.Snapshot()
		return err == nil && n.App.EvmKeeper.GetNonce(ctx, sender.EvmAddress) == 1
	}, time.Second, time.Millisecond)
	sealed := make(chan struct {
		block tmtypes.SealedBlock
		err   error
	}, 1)
	go func() {
		block, err := n.ConsensusSource.Seal(t.Context(), 2)
		sealed <- struct {
			block tmtypes.SealedBlock
			err   error
		}{block, err}
	}()
	select {
	case result := <-sealed:
		t.Fatalf("SEAL overtook ACCEPT release: %v", result.err)
	case <-time.After(30 * time.Millisecond):
	}
	n.Replicator.mu.Unlock()
	unlocked = true
	result := <-accepted
	require.NoError(t, result.err)
	seal := <-sealed
	require.NoError(t, seal.err)
	require.Equal(t, tmtypes.Txs{tx}, seal.block.Txs)
	for _, log := range []*wal.WAL{n.WAL, p.cfg.WAL} {
		var kinds []string
		require.NoError(t, log.Scan(func(e wal.Entry) error {
			switch e.(type) {
			case *wal.Open:
				kinds = append(kinds, "OPEN")
			case *wal.Accept:
				kinds = append(kinds, "ACCEPT")
			case *wal.Seal:
				kinds = append(kinds, "SEAL")
			}
			return nil
		}))
		require.Equal(t, []string{"OPEN", "ACCEPT", "SEAL"}, kinds)
	}
	p.stop()
	reopenedWAL, err := wal.OpenWAL(p.dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(reopenedWAL.Close)
	exec, err := NewExecutor(p.backup, p.h.opts)
	require.NoError(t, err)
	r, err := NewReceipter(exec, p.backup, &p.backup.EvmKeeper, n.ReceiptKey, app.EVMReceiptModuleID)
	require.NoError(t, err)
	replay := NewAppReplayer(exec, r)
	recovered, err := reopenedWAL.Replay(replay)
	require.NoError(t, err)
	require.Equal(t, 1, recovered.Accepts)
	require.Equal(t, result.send.Receipt, recovered.LastAccept.Receipt)
	require.Equal(t, [][]byte{tx}, replay.batch.Txs)
	require.NoError(t, n.Stop())
	repair103ClearRuntime(n)
	cfg := n.Config
	cfg.StandbyAddress = ""
	restarted, err := NewNode(cfg, n.App)
	require.NoError(t, err)
	t.Cleanup(func() { _ = restarted.Stop() })
	require.NoError(t, restarted.Start(t.Context()))
	block, err := restarted.ConsensusSource.Seal(t.Context(), 2)
	require.NoError(t, err)
	require.Equal(t, seal.block, block)
	repair103Finalize(t, restarted.App, restarted.ConsensusSource, p.h.opts.ProposerAddress, 2, block)
}

func TestRepair_103_PromotionDrainsAcknowledgedTail(t *testing.T) {
	for _, handover := range []bool{false, true} {
		t.Run(map[bool]string{false: "failover", true: "handover"}[handover], func(t *testing.T) {
			sender := utils.NewSigner()
			p := repair103Replicated(t, sender)
			tx := consensusFenceTransfer(t, p.n.App, sender, 0)
			_, err := p.n.Accepter.Accept(tx)
			require.NoError(t, err)
			block, err := p.n.ConsensusSource.Seal(t.Context(), 2)
			require.NoError(t, err)
			// Finalize both sides, but delay the standby's local commit.
			request := &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: p.n.App.ChainID, Height: 2, Time: block.OpenTime, ProposerAddress: p.h.opts.ProposerAddress}, Txs: append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...)}
			_, err = p.backup.FinalizeBlock(t.Context(), request)
			require.NoError(t, err)
			p.n.App.SetCommitListener(nil)
			repair103Finalize(t, p.n.App, p.n.ConsensusSource, p.h.opts.ProposerAddress, 2, block)
			require.NoError(t, p.n.Driver.OnCommit(2, p.n.App.LastCommitID().Hash))
			next := consensusFenceTransfer(t, p.n.App, sender, 1)
			issued, err := p.n.Accepter.Accept(next)
			require.NoError(t, err)
			require.Equal(t, uint64(43), p.sb.LastSeq())
			p.sb.mu.Lock()
			require.Equal(t, uint64(44), p.sb.cur.lastSeq)
			require.NotEmpty(t, p.sb.pending)
			p.sb.mu.Unlock()
			promoted := make(chan struct {
				seq uint64
				err error
			}, 1)
			key := &ReceiptKey{receiptKeyState: &receiptKeyState{private: p.cfg.ReceiptKey, public: [32]byte(p.cfg.ReceiptKey.Public().(ed25519.PublicKey))}}
			id := sha256.Sum256(key.public[:])
			require.NoError(t, key.SetPins([]verify.SequencerAuthorization{{SequencerID: id, PublicKey: key.public, FirstBatchNumber: 1, LastBatchNumber: 100}}))
			go func() {
				if handover {
					result, err := Handover(t.Context(), HandoverConfig{Primary: p.n.Executor, Replicator: p.n.Replicator, Standby: p.sb, Key: key, OpenHeight: 3, ExpiryMs: uint64(time.Now().Add(time.Minute).UnixMilli())})
					promoted <- struct {
						seq uint64
						err error
					}{result.LastSeq, err}
				} else {
					result, err := Failover(p.sb, key, 3, uint64(time.Now().Add(time.Minute).UnixMilli()))
					promoted <- struct {
						seq uint64
						err error
					}{result.LastSeq, err}
				}
			}()
			require.Eventually(t, func() bool { p.sb.mu.Lock(); defer p.sb.mu.Unlock(); return p.sb.promoting }, time.Second, time.Millisecond)
			select {
			case result := <-promoted:
				t.Fatalf("leadership abandoned acknowledged tail: %+v", result)
			default:
			}
			p.backup.SetDeliverStateToCommit()
			_, err = p.backup.Commit(t.Context())
			require.NoError(t, err)
			select {
			case result := <-promoted:
				require.NoError(t, result.err)
				require.Equal(t, uint64(44), result.seq)
			case <-time.After(2 * time.Second):
				t.Fatal("promotion did not replay pending tail after commit")
			}
			ctx, err := p.replay.exec.Snapshot()
			require.NoError(t, err)
			require.Equal(t, uint64(2), p.backup.EvmKeeper.GetNonce(ctx, sender.EvmAddress))
			p.stop()
			w, err := wal.OpenWAL(p.dir, wal.DefaultConfig())
			require.NoError(t, err)
			t.Cleanup(w.Close)
			opts := p.h.opts
			opts.LastSeq = 43
			opts.LastSeqHash, err = p.backup.SequencerKeeper.LastSeqHash(p.backup.GetCheckCtx())
			require.NoError(t, err)
			exec, err := NewExecutor(p.backup, opts)
			require.NoError(t, err)
			r, err := NewReceipter(exec, p.backup, &p.backup.EvmKeeper, p.n.ReceiptKey, app.EVMReceiptModuleID)
			require.NoError(t, err)
			replayed, err := w.Replay(NewAppReplayer(exec, r))
			require.NoError(t, err)
			require.Equal(t, 1, replayed.Accepts)
			require.Equal(t, issued.Receipt, replayed.LastAccept.Receipt)
			require.Equal(t, uint64(44), replayed.LastAccept.Seq)
		})
	}
}

func TestRepair_103_ReceiptFirstPublicationAfterCommit(t *testing.T) {
	sender := utils.NewSigner()
	n, _ := repair103Node(t, sender, ModeSequential)
	require.NoError(t, n.Start(t.Context()))
	var sends []*evmrpc.SequencerSend
	var hashes []common.Hash
	for nonce := uint64(0); nonce < 3; nonce++ {
		accepted, err := n.Accepter.Accept(consensusFenceTransfer(t, n.App, sender, nonce))
		require.NoError(t, err)
		decoded, err := codec.DecodeReceipt(accepted.Receipt)
		require.NoError(t, err)
		hash := common.Hash(decoded.ActivityID)
		send, err := n.ReceiptSource.SequencerSend(hash)
		require.NoError(t, err)
		sends = append(sends, send)
		hashes = append(hashes, hash)
	}
	source := NewReceiptSource(1)
	// Commit wins before the first publication of a durable accepted send.
	source.Committed(2)
	source.Record(hashes[1], *sends[1])
	source.Record(hashes[2], *sends[2]) // evicts seq 44, not the never-published seq 43
	source.Record(hashes[0], *sends[0])
	got, err := source.SequencerSend(hashes[0])
	require.NoError(t, err)
	require.Equal(t, sends[0], got)
	source.Record(hashes[1], *sends[1])
	evicted, err := source.SequencerSend(hashes[1])
	require.NoError(t, err)
	require.Nil(t, evicted, "an evicted duplicate cannot replace a first publication")
	// Exercise concurrent watermark advancement and real signed-send publication.
	for i := 0; i < 32; i++ {
		source := NewReceiptSource(3)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); source.Committed(2) }()
		go func() { defer wg.Done(); source.Record(hashes[0], *sends[0]) }()
		wg.Wait()
		got, err := source.SequencerSend(hashes[0])
		require.NoError(t, err)
		require.Equal(t, sends[0], got)
	}
}

func TestRepair_103_ReplicationRejectsForgedACK(t *testing.T) {
	for _, peer := range []string{"plaintext", "untrusted-root", "wrong-key", "missing-trust"} {
		t.Run(peer, func(t *testing.T) {
			sender := utils.NewSigner()
			n, _ := repair103Node(t, sender, ModeSequential)
			require.NoError(t, n.Start(t.Context()))
			auth, server := Repair103TLSFixture(t)
			var l net.Listener
			var err error
			switch peer {
			case "plaintext", "missing-trust":
				l, err = net.Listen("tcp", "127.0.0.1:0")
			default:
				if peer == "untrusted-root" {
					_, server = Repair103TLSFixture(t)
				} else {
					auth.PeerSPKI = hex.EncodeToString(make([]byte, 32))
				}
				cfg, e := server.TLSConfig(true)
				require.NoError(t, e)
				l, err = tls.Listen("tcp", "127.0.0.1:0", cfg)
			}
			require.NoError(t, err)
			t.Cleanup(func() { _ = l.Close() })
			attackDone := make(chan struct{})
			go func() {
				defer close(attackDone)
				c, err := l.Accept()
				if err != nil {
					return
				}
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(time.Second))
				forged := Ack{Status: AckOK, LastSeq: 43}.encode()
				_, _ = c.Write(forged[:])
				_, _ = io.Copy(io.Discard, c)
			}()
			dial := DialStandby(l.Addr().String(), 200*time.Millisecond, auth)
			if peer == "missing-trust" {
				dial = DialStandby(l.Addr().String(), 200*time.Millisecond)
			}
			n.Replicator.Close()
			n.Replicator, err = NewReplicator(ReplicatorConfig{WAL: n.WAL, Dial: dial, ReplicationTimeout: 200 * time.Millisecond})
			require.NoError(t, err)
			n.Accepter.rep = n.Replicator
			accepted, err := n.Accepter.Accept(consensusFenceTransfer(t, n.App, sender, 0))
			require.ErrorIs(t, err, ErrReplicationHalted)
			require.Empty(t, accepted.Receipt)
			require.Empty(t, n.ReceiptSource.entries)
			_ = l.Close()
			select {
			case <-attackDone:
			case <-time.After(2 * time.Second):
				t.Fatal("adversarial peer did not stop")
			}
		})
	}
}

func TestRepair_103_StandbyRejectsUnauthorizedControl(t *testing.T) {
	sender := utils.NewSigner()
	p := repair103Replicated(t, sender)
	// The listener uses the same authentication wrapper as StartStandby.
	// Give each unauthorized peer the valid primary name but an untrusted key/root.
	auth, server := Repair103TLSFixture(t)
	cfg, err := server.TLSConfig(true)
	require.NoError(t, err)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- p.sb.Serve(standbyRoleListener{Listener: listener, ctx: ctx, timeout: 100 * time.Millisecond})
	}()
	defer func() { cancel(); _ = listener.Close(); <-done }()
	var before int
	require.NoError(t, p.cfg.WAL.Scan(func(wal.Entry) error { before++; return nil }))
	// Produce real control frames from the primary's durable WAL.
	var frames []wal.Frame
	require.NoError(t, p.n.WAL.Scan(func(e wal.Entry) error { f, err := wal.NewFrame(e); frames = append(frames, f); return err }))
	group, err := encodeGroup(frames)
	require.NoError(t, err)
	for _, attack := range []string{"plaintext", "no-client-certificate", "unknown-primary", "wrong-pin"} {
		t.Run(attack, func(t *testing.T) {
			var conn net.Conn
			var err error
			if attack == "plaintext" {
				conn, err = net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			} else {
				client, err := auth.TLSConfig(false)
				require.NoError(t, err)
				if attack == "no-client-certificate" {
					client.Certificates = nil
				}
				if attack == "unknown-primary" {
					rogue, _ := Repair103TLSFixture(t)
					pair, e := tls.LoadX509KeyPair(rogue.CertFile, rogue.KeyFile)
					require.NoError(t, e)
					client.Certificates = []tls.Certificate{pair}
				}
				if attack == "wrong-pin" {
					pair, e := tls.LoadX509KeyPair(server.CertFile, server.KeyFile)
					require.NoError(t, e)
					client.Certificates = []tls.Certificate{pair}
				}
				d := tls.Dialer{Config: client, NetDialer: &net.Dialer{Timeout: time.Second}}
				conn, err = d.DialContext(t.Context(), "tcp", listener.Addr().String())
			}
			if err == nil {
				defer conn.Close()
				require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
				_, _ = conn.Write(group)
				var ack [ackSize]byte
				_, err = io.ReadFull(conn, ack[:])
				require.Error(t, err, "unauthorized lifecycle record received an acknowledgement")
			}
		})
	}
	// A trusted producer still reaches the consumer after hostile connections.
	conn, err := DialStandby(listener.Addr().String(), time.Second, auth)(t.Context())
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
	_, err = conn.Write(group)
	require.NoError(t, err)
	var raw [ackSize]byte
	_, err = io.ReadFull(conn, raw[:])
	require.NoError(t, err)
	ack, err := decodeAck(raw)
	require.NoError(t, err)
	require.Equal(t, AckOK, ack.Status)
	var after int
	require.NoError(t, p.cfg.WAL.Scan(func(wal.Entry) error { after++; return nil }))
	require.Equal(t, before, after, "unknown peers must not append even unsigned lifecycle records")
}
