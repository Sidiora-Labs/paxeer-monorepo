package sequencer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/holiman/uint256"
)

func (n *Node) Start(ctx context.Context) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n == nil || n.App == nil {
		return errors.New("sequencer: start needs an assembled node")
	}
	if n.Standby != nil {
		return nil
	}
	if n.WAL == nil || n.Executor == nil || n.FeeFloor == nil || n.ReceiptKey == nil ||
		n.Receipter == nil || n.Accepter == nil || n.Replicator == nil || n.Driver == nil ||
		n.ConsensusSource == nil || n.ReceiptSource == nil || n.ReceiptLogFeed == nil ||
		n.LogFeedPublisher == nil || n.Metrics == nil {
		return errors.New("sequencer: start needs an assembled primary")
	}
	n.Accepter.mu.Lock()
	defer n.Accepter.mu.Unlock()
	if n.Accepter.halted {
		return ErrReplicationHalted
	}
	if n.App.SequencerSource() != nil {
		return errors.New("sequencer: runtime already installed")
	}
	if _, err := n.Executor.Snapshot(); !errors.Is(err, ErrLayerNotOpen) {
		if err != nil {
			return err
		}
		return ErrLayerOpen
	}
	height := n.App.LastBlockHeight()
	if height < 0 || height == math.MaxInt64 {
		return errors.New("sequencer: invalid committed height")
	}
	state := n.App.NewUncachedContext(false, tmproto.Header{ChainID: n.App.ChainID, Height: height})
	pins, err := n.App.SequencerKeeper.ReceiptKeyPins(state)
	if err != nil {
		return err
	}
	info, err := n.App.UpgradeKeeper.GetUpgradeInfoPath()
	if err != nil {
		return fmt.Errorf("sequencer: app home: %w", err)
	}
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	key, err := LoadReceiptKey(n.Config.ReceiptKeyFile, consensus.KeyFile(), pins, height+1)
	if err != nil {
		return err
	}
	if err := n.reconcileWALTip(ctx, height); err != nil {
		return err
	}
	*n.ReceiptKey = *key
	defer func() {
		if err != nil {
			n.Accepter.halted = true
			_ = n.Executor.Fence()
		}
	}()
	if err := n.ReceiptSource.restoreCommitted(ctx, n.WAL); err != nil {
		return fmt.Errorf("sequencer: start receipt source rebuild: %w", err)
	}
	replayer := &nodeStartReplayer{AppReplayer: NewAppReplayer(n.Executor, n.Receipter), ctx: ctx}
	replayer.src = n.ReceiptSource
	result, err := n.WAL.Replay(replayer)
	if err != nil {
		return fmt.Errorf("sequencer: start WAL replay: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := n.Executor.Snapshot(); errors.Is(err, ErrLayerNotOpen) {
		if replayer.sealed {
			if replayer.batch == nil || replayer.batch.Height != height+1 {
				return errors.New("sequencer: sealed WAL tail differs from application tip")
			}
			n.ConsensusSource.pending = replayer.batch
		} else {
			openTime := time.Now().UnixMilli()
			if err := n.Executor.Open(height+1, openTime); err != nil {
				return err
			}
			baseFee, err := n.FeeFloor.BaseFeePerGas()
			if err != nil {
				return err
			}
			if baseFee.IsNegative() {
				return errors.New("sequencer: negative open base fee")
			}
			fee, overflow := uint256.FromBig(baseFee.TruncateInt().BigInt())
			if overflow {
				return errors.New("sequencer: open base fee exceeds uint256")
			}
			ack, err := n.Replicator.Release(ctx, &wal.Open{
				Height: uint64(height + 1), OpenTimeMs: uint64(openTime), BaseFee: fee,
			})
			if err != nil {
				return err
			}
			if ack.Status != AckOK {
				return fmt.Errorf("%w: OPEN status %d", ErrStandbyRefused, ack.Status)
			}
		}
	} else if err != nil {
		return err
	}
	if result.LastAccept != nil {
		n.Metrics.SetOpenSeq(result.LastAccept.Seq)
	}
	n.App.SetOpenBlock(n.Executor)
	n.App.SetSequencerSource(n.ConsensusSource)
	n.App.SetSequencerRPC(evmrpc.NewSequencerOption(n.Accepter, n.ReceiptLogFeed),
		evmrpc.SequencerReceiptOption{Source: n.ReceiptSource, Keeper: &n.App.SequencerKeeper})
	n.App.SetCommitListener(func(height int64, appHash []byte) error {
		if err := n.Driver.OnCommit(height, appHash); err != nil {
			n.Accepter.mu.Lock()
			n.Accepter.halted = true
			n.Accepter.mu.Unlock()
			_ = n.Executor.Fence()
			return fmt.Errorf("sequencer: commit listener at height %d: %w", height, err)
		}
		n.ReceiptSource.Committed(height)
		return n.LogFeedPublisher.Committed(height)
	})
	return nil
}

type nodeStartReplayer struct {
	*AppReplayer
	ctx    context.Context
	sealed bool
}

func (r *nodeStartReplayer) Apply(e wal.Entry) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if err := r.AppReplayer.Apply(e); err != nil {
		return err
	}
	switch e.(type) {
	case *wal.Open:
		r.sealed = false
	case *wal.Seal:
		r.sealed = true
	}
	return nil
}

func (r *nodeStartReplayer) Execute(a *wal.Accept) ([]byte, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	return r.AppReplayer.Execute(a)
}

// A crash can leave SEAL durable after the application committed but before
// Driver wrote COMMITTED. Only the exact sequence/hash tip can reconcile it.
func (n *Node) reconcileWALTip(ctx context.Context, height int64) error {
	state := n.App.GetCheckCtx()
	lastSeq, err := n.App.SequencerKeeper.LastSeq(state)
	if err != nil {
		return err
	}
	lastHash, err := n.App.SequencerKeeper.LastSeqHash(state)
	if err != nil {
		return err
	}
	committed := n.WAL.LastCommitted()
	if committed != nil {
		if committed.Height > uint64(height) {
			return errors.New("sequencer: WAL committed tip is ahead of app")
		}
		if committed.Height == uint64(height) {
			if committed.LastSeq != lastSeq || !bytes.Equal(committed.AppHash[:], n.App.CommitMultiStore().LastCommitID().Hash) {
				return errors.New("sequencer: WAL committed tip differs from app")
			}
			return nil
		}
	}
	var seal *wal.Seal
	if err := n.WAL.Scan(func(e wal.Entry) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if v, ok := e.(*wal.Seal); ok && v.Height <= uint64(height) {
			seal = v
		}
		return nil
	}); err != nil {
		return err
	}
	if seal == nil && committed == nil {
		return nil
	}
	if seal == nil || seal.Height != uint64(height) || seal.ToSeq != lastSeq || seal.LastSeqHash != lastHash {
		return errors.New("sequencer: sealed WAL tip differs from committed app")
	}
	hash := n.App.CommitMultiStore().LastCommitID().Hash
	if len(hash) != 32 {
		return errors.New("sequencer: committed app hash is not 32 bytes")
	}
	_, err = n.Replicator.Release(ctx, &wal.Committed{Height: uint64(height), LastSeq: lastSeq, AppHash: [32]byte(hash)})
	return err
}
