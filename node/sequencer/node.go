package sequencer

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/ante"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"net"
	"path/filepath"
	"sync"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	"go.opentelemetry.io/otel"
)

type Node struct {
	Config           Config
	App              *app.App
	WAL              *wal.WAL
	Replicator       *Replicator
	Executor         Executor
	FeeFloor         *FeeFloor
	ReceiptKey       *ReceiptKey
	Receipter        *Receipter
	Accepter         *Accepter
	ConsensusSource  *ConsensusSource
	Driver           *Driver
	ReceiptSource    *ReceiptSource
	ReceiptLogFeed   *evmrpc.ReceiptLogFeed
	LogFeedPublisher *LogFeedPublisher
	NonceAdmission   *NonceAdmission
	Metrics          *Metrics
	Standby          *Standby
	cancel           context.CancelFunc
	LeaseHolder      *LeaseHolder
	leaseDone        chan struct{}
	lifecycle        sync.Mutex
}

func NewNode(cfg Config, a *app.App) (_ *Node, err error) {
	if !cfg.Enable {
		return nil, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("%w: sequencer needs an app", ErrConfigValue)
	}
	n := &Node{Config: cfg, App: a}
	if cfg.Role == RoleStandby {
		ctx, cancel := context.WithCancel(context.Background())
		n.Standby, err = StartStandby(ctx, cfg, a)
		if err != nil {
			cancel()
			return nil, err
		}
		n.cancel = cancel
		n.WAL = n.Standby.wal
		return n, nil
	}
	var dial func(context.Context) (net.Conn, error)
	if cfg.StandbyAddress != "" {
		if _, _, err := net.SplitHostPort(cfg.StandbyAddress); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConfigStandbyAddress, err)
		}
		if _, err := cfg.ReplicationTLS.TLSConfig(false); err != nil {
			return nil, err
		}
		dial = DialStandby(cfg.StandbyAddress, cfg.ReplicationTimeout, cfg.ReplicationTLS)
	}
	n.WAL, err = wal.OpenWAL(cfg.WALDir, wal.DefaultConfig())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if n.Replicator != nil {
				n.Replicator.Close()
			}
			n.WAL.Close()
		}
	}()
	n.Replicator, err = NewReplicator(ReplicatorConfig{
		WAL: n.WAL, Dial: dial, Standalone: cfg.StandbyAddress == "",
		ReplicationTimeout: cfg.ReplicationTimeout,
	})
	if err != nil {
		return nil, err
	}
	infoPath, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	if err != nil {
		return nil, fmt.Errorf("sequencer: app home: %w", err)
	}
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(infoPath))).PrivValidator
	pv, err := privval.LoadFilePVEmptyState(consensus.KeyFile(), "")
	if err != nil {
		return nil, fmt.Errorf("sequencer: consensus key: %w", err)
	}
	height := a.LastBlockHeight()
	state := a.NewUncachedContext(false, tmproto.Header{ChainID: a.ChainID, Height: height})
	pins, err := a.SequencerKeeper.ReceiptKeyPins(state)
	if err != nil {
		return nil, err
	}
	n.ReceiptKey, err = LoadReceiptKey(cfg.ReceiptKeyFile, consensus.KeyFile(), pins, height+1)
	if err != nil {
		return nil, err
	}
	lastSeq, err := a.SequencerKeeper.LastSeq(state)
	if err != nil {
		return nil, err
	}
	lastHash, err := a.SequencerKeeper.LastSeqHash(state)
	if err != nil {
		return nil, err
	}
	exec, err := NewExecutor(a, Options{
		Mode: cfg.Mode, ChainID: a.ChainID, ProposerAddress: pv.Key.PubKey.Address(),
		LastSeq: lastSeq, LastSeqHash: lastHash,
		MicroBatch: MicroBatchOptions{Window: cfg.MicroBatchWindow, MaxCount: cfg.MicroBatchMaxCount},
	})
	if err != nil {
		return nil, err
	}
	n.FeeFloor, err = NewFeeFloor(exec, a, a.GetCheckCtx().MinGasPrices().String(),
		a.ParamsKeeper, a.OracleKeeper, a.PricefeedKeeper, &a.EvmKeeper)
	if err != nil {
		return nil, err
	}
	n.Executor = n.FeeFloor
	n.Receipter, err = NewReceipter(n.Executor, a, &a.EvmKeeper, n.ReceiptKey, app.EVMReceiptModuleID)
	if err != nil {
		return nil, err
	}
	n.ConsensusSource, err = NewConsensusSource(a, n.Executor, n.ReceiptKey, WithConsensusReplicator(n.Replicator))
	if err != nil {
		return nil, err
	}
	n.Metrics, err = NewMetrics(otel.Meter("github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"))
	if err != nil {
		return nil, err
	}
	n.Metrics.SetOpenSeq(lastSeq)
	n.Metrics.SetSealedHeight(height)
	n.ReceiptSource = &ReceiptSource{}
	n.ReceiptSource.Committed(height)
	n.ReceiptLogFeed = evmrpc.NewReceiptLogFeed(height)
	n.LogFeedPublisher = NewLogFeedPublisher(n.ReceiptLogFeed)
	n.Accepter = NewAccepter(n.Receipter, n.Replicator, n.ReceiptSource, n.Metrics)
	n.ConsensusSource.SetAccepter(n.Accepter)
	n.NonceAdmission, err = NewNonceAdmission(cfg, nodeOpenNonces{a},
		func(_ common.Address, _ uint64, tx []byte) error {
			_, err := n.Accepter.Accept(tx)
			return err
		}, cfg.LeaseDuration)
	if err != nil {
		return nil, err
	}
	n.Driver, err = NewDriver(DriverConfig{
		App: a, Executor: n.FeeFloor, Replicator: n.Replicator,
		ReceiptKey: n.ReceiptKey, NonceAdmission: n.NonceAdmission,
		NonceQueue: n.NonceAdmission.queue, Metrics: n.Metrics, WALRetainBlocks: cfg.WALRetainBlocks,
	})
	if err != nil {
		return nil, err
	}
	n.ReceiptSource.pending = n.pendingSend
	return n, nil
}

type nodeOpenNonces struct{ app *app.App }

func (r nodeOpenNonces) EvmNonce(addr common.Address) uint64 {
	return r.app.EvmKeeper.GetNonce(r.app.LatestCtx(), addr)
}

var ErrNonceHeld = errors.New("sequencer: transaction held at submitted awaiting nonce")

// Accept is the verified RPC admission adapter; queued transactions never execute
// until their sender's exact next nonce is available.
func (n *Node) Accept(txBytes []byte) (evmrpc.AcceptedSend, error) {
	n.Accepter.mu.Lock()
	err := n.Accepter.admission()
	n.Accepter.mu.Unlock()
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	tx, err := n.App.GetTxConfig().TxDecoder()(txBytes)
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	msg := n.App.GetEVMMsg(tx)
	if msg == nil {
		return n.Accepter.Accept(txBytes)
	}
	etx, _ := msg.AsTransaction()
	if etx == nil {
		return evmrpc.AcceptedSend{}, inputErr("missing EVM transaction")
	}
	if _, err := ante.ResolveTransactionChain(n.App.LatestCtx(), etx, n.App.EvmKeeper.ChainID(n.App.GetCheckCtx()), n.App.LayerXChainsKeeper); err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(etx.ChainId()), etx)
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	held, err := n.NonceAdmission.Send(sender, etx.Nonce(), txBytes)
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	if held {
		return evmrpc.AcceptedSend{}, ErrNonceHeld
	}
	send, err := n.ReceiptSource.SequencerSend(etx.Hash())
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	if send == nil || len(send.Receipt) == 0 {
		return evmrpc.AcceptedSend{}, ErrReceiptInvariant
	}
	decoded, err := codec.DecodeReceipt(send.Receipt)
	if err != nil {
		return evmrpc.AcceptedSend{}, err
	}
	n.ReceiptSource.mu.RLock()
	entry := n.ReceiptSource.entries[etx.Hash()]
	n.ReceiptSource.mu.RUnlock()
	if entry.exec == nil {
		return evmrpc.AcceptedSend{}, ErrReceiptInvariant
	}
	rcpt := *entry.exec
	return evmrpc.AcceptedSend{Receipt: send.Receipt, Height: int64(binary.BigEndian.Uint64(decoded.BatchID[24:])), Exec: &rcpt}, nil

}

func (n *Node) pendingSend(hash common.Hash) (*evmrpc.SequencerSend, error) {
	for _, failure := range n.NonceAdmission.queue.Failures() {
		tx, err := n.App.GetTxConfig().TxDecoder()(failure.Tx)
		if err != nil {
			return nil, err
		}
		if msg := n.App.GetEVMMsg(tx); msg != nil {
			etx, _ := msg.AsTransaction()
			if etx != nil && etx.Hash() == hash {
				return nil, failure.Err
			}
		}
	}
	for _, h := range n.NonceAdmission.queue.Pending() {
		tx, err := n.App.GetTxConfig().TxDecoder()(h.Tx)
		if err != nil {
			return nil, err
		}
		if msg := n.App.GetEVMMsg(tx); msg != nil {
			etx, _ := msg.AsTransaction()
			if etx != nil && etx.Hash() == hash {
				return &evmrpc.SequencerSend{Nonce: h.Nonce}, nil
			}
		}
	}
	return nil, nil
}

// Promote is the operator-triggered takeover of this pinned standby's durable WAL.
// It fences and drains the replicated tail before replacing the running pipeline.
func (n *Node) Promote(ctx context.Context) error {
	n.lifecycle.Lock()
	defer n.lifecycle.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if n.Standby == nil {
		return errors.New("sequencer: promotion requires a running standby")
	}
	info, err := n.App.UpgradeKeeper.GetUpgradeInfoPath()
	if err != nil {
		return err
	}
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	consensusPV, err := privval.LoadFilePVEmptyState(consensus.KeyFile(), "")
	if err != nil {
		return err
	}
	leaders, err := n.App.SequencerKeeper.Leaders(n.App.GetCheckCtx())
	if err != nil {
		return err
	}
	authorized := false
	for _, leader := range leaders {
		authorized = authorized || bytes.Equal(consensusPV.Key.PubKey.Bytes(), leader[:])
	}
	if !authorized {
		return errors.New("sequencer: takeover consensus key is no longer governed")
	}
	pins, err := n.App.SequencerKeeper.ReceiptKeyPins(n.App.GetCheckCtx())
	if err != nil {
		return err
	}
	height := n.App.LastBlockHeight() + 1
	key, err := LoadReceiptKey(n.Config.ReceiptKeyFile, consensus.KeyFile(), pins, height)
	if err != nil {
		return err
	}
	s := n.Standby
	historical, ok := s.exec.(*standbyRoleReplay)
	if !ok {
		return errors.New("sequencer: standby is missing its authorized replay pipeline")
	}
	if !bytes.Equal(key.private, s.receiptKey) {
		return ErrReceiptKeyNotPinned
	}
	if _, err := Failover(s, key, height, uint64(time.Now().Add(n.Config.LeaseDuration).UnixMilli())); err != nil {
		return err
	}
	if n.cancel != nil {
		n.cancel()
	}
	if s.serveDone != nil {
		select {
		case <-s.serveDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.wal.Close()
	n.App.SetOpenBlock(nil)
	n.App.SetSequencerSource(nil)
	n.App.SetCommitListener(nil)
	n.App.SetSequencerRPC(evmrpc.SequencerOption{}, evmrpc.SequencerReceiptOption{})
	cfg := n.Config
	cfg.Role = RolePrimary
	cfg.StandbyAddress = ""
	promoted, err := NewNode(cfg, n.App)
	if err != nil {
		return err
	}
	// Older OPEN records do not carry their proposer. The running standby
	// retains the original primary proposer for those historical records.
	oldFloor := historical.exec.(*FeeFloor)
	oldSeq := baseSequential(oldFloor.Executor)
	currentSeq := baseSequential(promoted.FeeFloor.Executor)
	currentSeq.proposer = append([]byte(nil), oldSeq.proposer...)
	if err := promoted.Start(ctx); err != nil {
		_ = promoted.Stop()
		return err
	}
	pv, err := privval.LoadFilePVEmptyState(consensus.KeyFile(), "")
	if err != nil {
		_ = promoted.Stop()
		return err
	}
	currentSeq.mu.Lock()
	currentSeq.proposer = append([]byte(nil), pv.Key.PubKey.Address()...)
	currentSeq.mu.Unlock()
	// Keep the receiver stable: installed RPC and commit callbacks own promoted.
	// Its resources are also retained here for operator stop and inspection.
	n.Config, n.WAL, n.Replicator = promoted.Config, promoted.WAL, promoted.Replicator
	n.Executor, n.FeeFloor, n.ReceiptKey = promoted.Executor, promoted.FeeFloor, promoted.ReceiptKey
	n.Receipter, n.Accepter, n.ConsensusSource = promoted.Receipter, promoted.Accepter, promoted.ConsensusSource
	n.Driver, n.ReceiptSource, n.ReceiptLogFeed = promoted.Driver, promoted.ReceiptSource, promoted.ReceiptLogFeed
	n.LogFeedPublisher, n.NonceAdmission, n.Metrics = promoted.LogFeedPublisher, promoted.NonceAdmission, promoted.Metrics
	n.LeaseHolder, n.leaseDone, n.cancel = promoted.LeaseHolder, promoted.leaseDone, promoted.cancel
	n.Standby = nil
	return nil
}

func baseSequential(exec Executor) *sequential {
	switch e := exec.(type) {
	case *microBatch:
		return e.sequential
	case *sequential:
		return e
	default:
		panic("sequencer: unknown execution pipeline")
	}
}

func (n *Node) startLease(ctx context.Context, result wal.ReplayResult) error {
	keyID, err := n.ReceiptKey.KeyID(n.App.LastBlockHeight() + 1)
	if err != nil {
		return err
	}
	holder, err := NewLeaseHolder(LeaseHolderConfig{Replicator: n.Replicator, Replay: result, KeyID: keyID, LeaseDuration: n.Config.LeaseDuration})
	if err != nil {
		return err
	}
	if err := holder.Start(ctx); err != nil {
		return err
	}
	n.LeaseHolder, n.Accepter.lease = holder, holder
	n.Driver.cfg.LeaseHolder = holder
	run, cancel := context.WithCancel(ctx)
	n.cancel, n.leaseDone = cancel, make(chan struct{})
	go func() {
		defer close(n.leaseDone)
		ticker := time.NewTicker(max(time.Nanosecond, n.Config.LeaseDuration/4))
		defer ticker.Stop()
		for {
			select {
			case <-run.Done():
				return
			case <-ticker.C:
				now := time.Now()
				if !now.Before(holder.Expiry()) || holder.renew(run, now) != nil {
					n.Accepter.mu.Lock()
					n.Accepter.halted = true
					_ = n.Executor.Fence()
					n.Accepter.mu.Unlock()
					return
				}
			}
		}
	}()
	return nil
}
