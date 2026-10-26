package sequencer

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// ErrNotSequenced refuses a seal of a block that is not sequenced.
var ErrNotSequenced = errors.New("sequencer: height is not sequenced")

var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)

// ConsensusSource is the consensus SequencerSource over an executor: it seals the open layer,
// signs its MsgSequenceBatch with the pinned receipt key and seals it as tx 0 through SealBatch.
type ConsensusSource struct {
	app      *app.App
	exec     Executor
	key      *ReceiptKey
	repl     *Replicator
	mu       sync.Mutex
	pending  *Batch
	prepared *SealedLayer
	sealed   *SealedLayer
}

type ConsensusSourceOption func(*ConsensusSource) error

func WithConsensusReplicator(repl *Replicator) ConsensusSourceOption {
	return func(s *ConsensusSource) error {
		if repl == nil {
			return errors.New("sequencer: consensus source needs a replicator")
		}
		s.repl = repl
		return nil
	}
}

// NewConsensusSource builds the consensus source over exec on a, signing tx 0 with key.
func NewConsensusSource(a *app.App, exec Executor, key *ReceiptKey, opts ...ConsensusSourceOption) (*ConsensusSource, error) {
	if a == nil || exec == nil || key == nil {
		return nil, errors.New("sequencer: consensus source needs an app, executor and receipt key")
	}
	s := &ConsensusSource{app: a, exec: exec, key: key}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Sequenced reports whether the block at height is sequenced, read from the committed state
// the next block executes on.
func (s *ConsensusSource) Sequenced(height int64) bool {
	return s.app.Sequenced(s.app.GetCheckCtx(), height)
}

// Sealed returns L_N of the last Seal, or nil before the first.
func (s *ConsensusSource) Sealed() *SealedLayer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sealed
}

// Seal seals the open layer at height, builds and signs its MsgSequenceBatch with txRoot from
// App.SequencedTxRoot and seals it through SealBatch. The block carries the encoded batch at
// tx 0, the sealed transactions in seq order and the open time of the layer.
func (s *ConsensusSource) Seal(ctx context.Context, height int64) (tmtypes.SealedBlock, error) {
	if err := ctx.Err(); err != nil {
		return tmtypes.SealedBlock{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Sequenced(height) {
		return tmtypes.SealedBlock{}, fmt.Errorf("%w: height %d", ErrNotSequenced, height)
	}
	keyID, err := s.key.KeyID(height)
	if err != nil {
		return tmtypes.SealedBlock{}, err
	}
	batch := s.pending
	if batch == nil {
		batch, err = s.exec.Seal(height)
		if err != nil {
			return tmtypes.SealedBlock{}, err
		}
		s.pending = batch
	} else if batch.Height != height {
		return tmtypes.SealedBlock{}, fmt.Errorf("%w: got %d, want %d", ErrSealHeight, height, batch.Height)
	}
	if s.prepared == nil {
		typed := make([]sdk.Tx, len(batch.Txs))
		decode := s.app.GetTxConfig().TxDecoder()
		for i, tx := range batch.Txs {
			if typed[i], err = decode(tx); err != nil {
				return tmtypes.SealedBlock{}, fmt.Errorf("%w: seq %d: %v", ErrUndecodableTx, batch.FromSeq+uint64(i), err) //nolint:gosec // i indexes the batch
			}
		}
		fields := BatchFields(batch)
		txRoot := s.app.SequencedTxRoot(batch.Txs, typed)
		msg := seqtypes.MsgSequenceBatch{
			Height:      fields.Height,
			FromSeq:     fields.FromSeq,
			ToSeq:       fields.ToSeq,
			PrevSeqHash: fields.PrevSeqHash[:],
			LastSeqHash: fields.LastSeqHash[:],
			TxRoot:      txRoot[:],
			KeyId:       keyID[:],
		}
		digest, err := msg.SignBytes(batch.Ctx.ChainID())
		if err != nil {
			return tmtypes.SealedBlock{}, err
		}
		msg.Sig = ed25519.Sign(s.key.private, digest[:])
		sealed, _, err := SealBatch(s.app, batch, msg)
		if err != nil {
			return tmtypes.SealedBlock{}, err
		}
		s.prepared = sealed
	}
	sealed := s.prepared
	if s.repl != nil {
		fields := sealed.Batch
		if _, err := s.repl.Release(ctx, &wal.Seal{
			Height:      fields.Height,
			FromSeq:     fields.FromSeq,
			ToSeq:       fields.ToSeq,
			LastSeqHash: fields.LastSeqHash,
		}); err != nil {
			return tmtypes.SealedBlock{}, err
		}
	}
	s.sealed = sealed
	s.pending = nil
	s.prepared = nil
	txs := make(tmtypes.Txs, len(batch.Txs))
	for i, tx := range batch.Txs {
		txs[i] = tx
	}
	return tmtypes.SealedBlock{
		Batch:    sealed.Txs[0],
		Txs:      txs,
		OpenTime: time.UnixMilli(batch.TimestampMs).UTC(),
	}, nil
}
