package sequencer

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/holiman/uint256"
)

var ErrDriverHalted = errors.New("sequencer: block driver halted")

type DriverConfig struct {
	App             *app.App
	Executor        *FeeFloor
	Replicator      *Replicator
	NonceQueue      *NonceQueue
	Metrics         *Metrics
	WALRetainBlocks uint64
}

type Driver struct {
	mu     sync.Mutex
	cfg    DriverConfig
	halted error
}

func NewDriver(cfg DriverConfig) (*Driver, error) {
	if cfg.App == nil || cfg.Executor == nil || cfg.Replicator == nil || cfg.NonceQueue == nil || cfg.Metrics == nil {
		return nil, errors.New("sequencer: driver needs an app, fee-floor executor, replicator, nonce queue and metrics")
	}
	if cfg.WALRetainBlocks == 0 {
		return nil, errors.New("sequencer: driver WAL retention must be positive")
	}
	r := cfg.Replicator
	r.mu.Lock()
	previous := r.onWALSync
	r.onWALSync = func(d time.Duration) {
		cfg.Metrics.ObserveWALSync(d)
		if previous != nil {
			previous(d)
		}
	}
	r.mu.Unlock()
	return &Driver{cfg: cfg}, nil
}

func (d *Driver) OnCommit(height int64, appHash []byte) (err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.halted != nil {
		return fmt.Errorf("%w: %w", ErrDriverHalted, d.halted)
	}
	if height <= 0 || height == math.MaxInt64 || height != d.cfg.App.LastBlockHeight() {
		return fmt.Errorf("sequencer: driver commit height %d does not name the committed tip", height)
	}
	if len(appHash) != 32 {
		return fmt.Errorf("sequencer: driver app hash is %d bytes, want 32", len(appHash))
	}
	if _, snapshotErr := d.cfg.Executor.Snapshot(); !errors.Is(snapshotErr, ErrLayerNotOpen) {
		if snapshotErr != nil {
			return snapshotErr
		}
		return ErrLayerOpen
	}
	lastSeq, err := d.cfg.App.SequencerKeeper.LastSeq(d.cfg.App.GetCheckCtx())
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			d.halted = err
		}
	}()
	committed := &wal.Committed{Height: uint64(height), LastSeq: lastSeq, AppHash: [32]byte(appHash)}
	if _, err = d.cfg.Replicator.Release(context.Background(), committed); err != nil {
		return err
	}
	d.cfg.Metrics.SetSealedHeight(height)
	var retainHeight uint64
	if committed.Height > d.cfg.WALRetainBlocks {
		retainHeight = committed.Height - d.cfg.WALRetainBlocks
	}
	if _, err = d.cfg.Replicator.wal.Prune(retainHeight); err != nil {
		return err
	}
	openTime := time.Now().UnixMilli()
	if err = d.cfg.Executor.Open(height+1, openTime); err != nil {
		return err
	}
	baseFee, err := d.cfg.Executor.BaseFeePerGas()
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
	if _, err = d.cfg.Replicator.Release(context.Background(), &wal.Open{
		Height: uint64(height + 1), OpenTimeMs: uint64(openTime), BaseFee: fee,
	}); err != nil {
		return err
	}
	d.cfg.NonceQueue.Sealed()
	d.cfg.Metrics.SetOpenSeq(lastSeq)
	return nil
}
