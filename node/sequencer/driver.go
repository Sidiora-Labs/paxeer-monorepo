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
	ReceiptKey      *ReceiptKey
	NonceAdmission  *NonceAdmission
	LeaseHolder     *LeaseHolder
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
	if err := d.cfg.App.CheckCommittedAppHash(height, appHash); err != nil {
		return fmt.Errorf("sequencer: driver commit: %w", err)
	}
	committedHash, err := d.cfg.App.CommittedAppHash(height)
	if err != nil {
		return fmt.Errorf("sequencer: driver commit: %w", err)
	}
	if _, snapshotErr := d.cfg.Executor.Snapshot(); !errors.Is(snapshotErr, ErrLayerNotOpen) {
		if snapshotErr != nil {
			return fmt.Errorf("sequencer: driver executor snapshot: %w", snapshotErr)
		}
		return fmt.Errorf("sequencer: driver executor snapshot: %w", ErrLayerOpen)
	}
	lastSeq, err := d.cfg.App.SequencerKeeper.LastSeq(d.cfg.App.GetCheckCtx())
	if err != nil {
		return fmt.Errorf("sequencer: driver last sequence: %w", err)
	}
	defer func() {
		if err != nil {
			d.halted = err
		}
	}()
	committed := &wal.Committed{Height: uint64(height), LastSeq: lastSeq, AppHash: [32]byte(committedHash)}
	if _, err = d.cfg.Replicator.Release(context.Background(), committed); err != nil {
		return fmt.Errorf("sequencer: driver release committed: %w", err)
	}
	d.cfg.Metrics.SetSealedHeight(height)
	var retainHeight uint64
	if committed.Height > d.cfg.WALRetainBlocks {
		retainHeight = committed.Height - d.cfg.WALRetainBlocks
	}
	if _, err = d.cfg.Replicator.wal.Prune(retainHeight); err != nil {
		return fmt.Errorf("sequencer: driver prune: %w", err)
	}
	if d.cfg.ReceiptKey != nil {
		pins, pinErr := d.cfg.App.SequencerKeeper.ReceiptKeyPins(d.cfg.App.GetCheckCtx())
		if pinErr != nil {
			return pinErr
		}
		if err = d.cfg.ReceiptKey.SetPins(pins); err != nil {
			return err
		}
		id, pinErr := d.cfg.ReceiptKey.KeyID(height + 1)
		if pinErr != nil {
			return pinErr
		}
		if d.cfg.LeaseHolder != nil {
			d.cfg.LeaseHolder.mu.Lock()
			d.cfg.LeaseHolder.keyID = id
			d.cfg.LeaseHolder.mu.Unlock()
		}
	}
	openTime := time.Now().UnixMilli()
	if err = d.cfg.Executor.Open(height+1, openTime); err != nil {
		return fmt.Errorf("sequencer: driver executor open: %w", err)
	}
	baseFee, err := d.cfg.Executor.BaseFeePerGas()
	if err != nil {
		return fmt.Errorf("sequencer: driver base fee: %w", err)
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
		ProposerAddress: baseSequential(d.cfg.Executor.Executor).proposer,
	}); err != nil {
		return fmt.Errorf("sequencer: driver release open at height %d: %w", height+1, err)
	}
	d.cfg.NonceQueue.Sealed()
	if d.cfg.NonceAdmission != nil {
		d.cfg.NonceAdmission.Open(time.Now())
	}
	d.cfg.Metrics.SetOpenSeq(lastSeq)
	return nil
}
