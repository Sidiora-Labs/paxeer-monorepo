package sequencer

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/paxeer-network/paxlog"
)

// ErrNotSequenced refuses a seal of a block that is not sequenced.
var ErrNotSequenced = errors.New("sequencer: height is not sequenced")
var ErrPromotionSeq = errors.New("sequencer: promotion sequence is below the stored sequence")

var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)

// ConsensusSource is the consensus SequencerSource over an executor: it seals the open layer,
// signs its MsgSequenceBatch with the pinned receipt key and seals it as tx 0 through SealBatch.
type ConsensusSource struct {
	app      *app.App
	exec     Executor
	key      *ReceiptKey
	repl     *Replicator
	accepter *Accepter
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

func NewPromotedConsensusSource(a *app.App, host Host, opts Options, lastSeq uint64, key *ReceiptKey) (*ConsensusSource, error) {
	if lastSeq < opts.LastSeq {
		return nil, fmt.Errorf("%w: got %d, want at least %d", ErrPromotionSeq, lastSeq, opts.LastSeq)
	}
	opts.LastSeq = lastSeq
	exec, err := NewExecutor(host, opts)
	if err != nil {
		return nil, err
	}
	return NewConsensusSource(a, exec, key)
}

// Sequenced reports whether the block at height is sequenced, read from the committed state
// the next block executes on.
func (s *ConsensusSource) Sequenced(height int64) bool {
	return s.app.Sequenced(s.app.GetCheckCtx(), height)
}

// SetAccepter installs the leader's execution, replication and receipt path.
func (s *ConsensusSource) SetAccepter(a *Accepter) {
	if a == nil {
		panic("sequencer: consensus source needs an accepter")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accepter = a
}

// Admit accepts mempool transactions in order, continuing past individual refusals.
func (s *ConsensusSource) Admit(ctx context.Context, height int64, txs tmtypes.Txs) (int, error) {
	admitted, _, err := s.AdmitWithAcknowledgements(ctx, height, txs)
	return admitted, err
}

// AdmitWithAcknowledgements also returns the processed prefix, including logged
// individual refusals. A closed height and a fatal error never acknowledge the
// unprocessed suffix. The lock prevents Seal from overtaking admission.
func (s *ConsensusSource) AdmitWithAcknowledgements(ctx context.Context, height int64, txs tmtypes.Txs) (admitted, processed int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if (s.pending != nil && s.pending.Height == height) || (s.sealed != nil && s.sealed.Height == height) {
		return 0, 0, nil
	}
	if s.accepter == nil {
		return 0, 0, fmt.Errorf("%w: consensus source has no accepter", ErrReplicationHalted)
	}
	log := paxlog.NewLogger("sequencer", "consensus-source")
	for _, tx := range txs {
		if err := ctx.Err(); err != nil {
			return admitted, processed, err
		}
		// Replay rebuilds the receipt source before proposal admission resumes. An
		// acceptance can reach the WAL before the proposal journal acknowledges it.
		id := sha256.Sum256(tx)
		if decoded, err := s.app.GetTxConfig().TxDecoder()(tx); err == nil {
			if msg := s.app.GetEVMMsg(decoded); msg != nil {
				if etx, _ := msg.AsTransaction(); etx != nil {
					id = etx.Hash()
				}
			}
		}
		s.accepter.src.mu.RLock()
		prior, exists := s.accepter.src.entries[id]
		s.accepter.src.mu.RUnlock()
		if exists && prior.height == height {
			admitted++
			processed++
			continue
		}
		if _, err := s.accepter.Accept(tx); err != nil {
			s.accepter.mu.Lock()
			halted := s.accepter.halted
			s.accepter.mu.Unlock()
			if halted {
				return admitted, processed, errors.Join(ErrReplicationHalted, err)
			}
			if errors.Is(err, ErrLayerNotOpen) || errors.Is(err, ErrLeaseFenced) || errors.Is(err, ErrReplicationHalted) {
				return admitted, processed, err
			}
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			log.Info("refused consensus mempool transaction", "height", height, "hash", fmt.Sprintf("%X", sha256.Sum256(tx)), "codespace", codespace, "code", code, "error", err.Error())
			processed++
			continue
		}
		admitted++
		processed++
	}
	return admitted, processed, nil
}

// Admit cannot execute on a standby; only an already sealed replicated height
// is available to a proposal retry.
func (s *standbyRoleSource) Admit(_ context.Context, height int64, _ tmtypes.Txs) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed != nil && s.sealed.Height == height {
		return 0, nil
	}
	return 0, ErrLayerNotOpen
}

// AdmitWithAcknowledgements on a standby processes no new transactions.
func (s *standbyRoleSource) AdmitWithAcknowledgements(ctx context.Context, height int64, txs tmtypes.Txs) (int, int, error) {
	admitted, err := s.Admit(ctx, height, txs)
	return admitted, 0, err
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
	if s.sealed != nil && s.sealed.Height == height {
		txs := make(tmtypes.Txs, len(s.sealed.Txs)-1)
		for i, tx := range s.sealed.Txs[1:] {
			txs[i] = append(tmtypes.Tx(nil), tx...)
		}
		return tmtypes.SealedBlock{Batch: append(tmtypes.Tx(nil), s.sealed.Txs[0]...), Txs: txs, OpenTime: s.sealed.Context().BlockTime()}, nil
	}
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
			if errors.Is(err, ErrFenced) {
				if fenceErr := s.exec.Fence(); fenceErr != nil {
					return tmtypes.SealedBlock{}, errors.Join(ErrLeaseFenced, fenceErr)
				}
				return tmtypes.SealedBlock{}, ErrLeaseFenced
			}
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
