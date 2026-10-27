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
	if committed := n.WAL.LastCommitted(); committed != nil {
		lastSeq, err := n.App.SequencerKeeper.LastSeq(state)
		if err != nil {
			return err
		}
		if committed.Height != uint64(height) || committed.LastSeq != lastSeq ||
			!bytes.Equal(committed.AppHash[:], n.App.CommitMultiStore().LastCommitID().Hash) {
			return errors.New("sequencer: WAL committed tip differs from app")
		}
	}
	*n.ReceiptKey = *key
	defer func() {
		if err != nil {
			n.Accepter.halted = true
			_ = n.Executor.Fence()
		}
	}()
	replayer := &nodeStartReplayer{AppReplayer: NewAppReplayer(n.Executor, n.Receipter), ctx: ctx}
	result, err := n.WAL.Replay(replayer)
	if err != nil {
		return fmt.Errorf("sequencer: start WAL replay: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := n.Executor.Snapshot(); errors.Is(err, ErrLayerNotOpen) {
		if replayer.sealed {
			return errors.New("sequencer: WAL ends in an uncommitted sealed block")
		}
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
		if _, err := n.Replicator.Release(ctx, &wal.Open{
			Height: uint64(height + 1), OpenTimeMs: uint64(openTime), BaseFee: fee,
		}); err != nil {
			return err
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
	n.App.SetCommitListener(func(height int64, appHash []byte) {
		if err := n.Driver.OnCommit(height, appHash); err != nil {
			n.Accepter.mu.Lock()
			n.Accepter.halted = true
			n.Accepter.mu.Unlock()
			_ = n.Executor.Fence()
			panic(err)
		}
		n.ReceiptSource.Committed(height)
		if err := n.LogFeedPublisher.Committed(height); err != nil {
			panic(err)
		}
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
