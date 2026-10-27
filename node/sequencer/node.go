package sequencer

import (
	"context"
	"fmt"
	"net"
	"path/filepath"

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
	if _, _, err := net.SplitHostPort(cfg.StandbyAddress); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConfigStandbyAddress, err)
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
		WAL: n.WAL, Dial: DialStandby(cfg.StandbyAddress, cfg.ReplicationTimeout),
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
		NonceQueue: n.NonceAdmission.queue, Metrics: n.Metrics, WALRetainBlocks: cfg.WALRetainBlocks,
	})
	if err != nil {
		return nil, err
	}
	return n, nil
}

type nodeOpenNonces struct{ app *app.App }

func (r nodeOpenNonces) EvmNonce(addr common.Address) uint64 {
	return r.app.EvmKeeper.GetNonce(r.app.LatestCtx(), addr)
}
