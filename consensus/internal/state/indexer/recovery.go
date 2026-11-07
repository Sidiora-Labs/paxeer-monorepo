package indexer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// RecoveryBlockStore supplies the committed blocks retained for replay.
type RecoveryBlockStore interface {
	Base() int64
	Height() int64
	LoadBlock(int64) *types.Block
}

// RecoveryStateStore supplies the committed height and persisted ABCI results.
// Block-store height alone may include a block whose application commit failed.
type RecoveryStateStore interface {
	Load() (sm.State, error)
	LoadFinalizeBlockResponses(int64) (*abci.ResponseFinalizeBlock, error)
}

// RecoveryArgs enables durable progress and startup replay. The caller owns all
// three stores and must retain blocks and responses beyond each sink's watermark.
// ProgressStore must be durable, and must be restored together with the sinks.
type RecoveryArgs struct {
	BlockStore    RecoveryBlockStore
	StateStore    RecoveryStateStore
	ProgressStore dbm.DB
}

func (r RecoveryArgs) validate() error {
	if r.BlockStore == nil || r.StateStore == nil || r.ProgressStore == nil {
		return errors.New("indexer recovery requires block, state and progress stores")
	}
	return nil
}

func indexedHeightKey(state sm.State, sink EventSinkType) []byte {
	key, err := orderedcode.Append(nil, "indexer/indexed-height/v1", state.ChainID, state.InitialHeight, string(sink))
	if err != nil {
		panic(err)
	}
	return key
}

func (r RecoveryArgs) loadHeight(state sm.State, sink EventSinkType) (int64, error) {
	data, err := r.ProgressStore.Get(indexedHeightKey(state, sink))
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return state.InitialHeight - 1, nil
	}
	if len(data) != 8 {
		return 0, errors.New("invalid indexed-height watermark")
	}
	height := int64(binary.BigEndian.Uint64(data))
	if height < state.InitialHeight-1 {
		return 0, errors.New("indexed-height watermark precedes initial height")
	}
	return height, nil
}

// IndexedHeight returns the last contiguous height durably indexed by sink.
// A missing watermark returns the predecessor of the chain's initial height.
func (r RecoveryArgs) IndexedHeight(sink EventSinkType) (int64, error) {
	if err := r.validate(); err != nil {
		return 0, err
	}
	state, err := r.StateStore.Load()
	if err != nil {
		return 0, err
	}
	if state.IsEmpty() || state.InitialHeight < 1 {
		return 0, errors.New("indexer recovery requires initialized chain state")
	}
	return r.loadHeight(state, sink)
}

func (is *Service) initRecovery() error {
	r, ok := is.recovery.Get()
	if !ok {
		logger.Warn("indexer durable replay is not configured")
		return nil
	}
	if err := r.validate(); err != nil {
		return err
	}
	state, err := r.StateStore.Load()
	if err != nil {
		return err
	}
	if state.IsEmpty() || state.InitialHeight < 1 {
		return errors.New("indexer recovery requires initialized chain state")
	}
	is.progressKeys = make(map[EventSinkType][]byte)
	is.indexed = make(map[EventSinkType]int64)
	for _, sink := range is.eventSinks {
		if sink.Type() == NULL {
			continue
		}
		if _, exists := is.indexed[sink.Type()]; exists {
			return fmt.Errorf("duplicate indexer sink %s", sink.Type())
		}
		height, err := r.loadHeight(state, sink.Type())
		if err != nil {
			return fmt.Errorf("loading %s indexed height: %w", sink.Type(), err)
		}
		if height > max(state.LastBlockHeight, state.InitialHeight-1) {
			return fmt.Errorf("%s indexed height %d exceeds committed state height %d", sink.Type(), height, state.LastBlockHeight)
		}
		is.indexed[sink.Type()] = height
		is.progressKeys[sink.Type()] = indexedHeightKey(state, sink.Type())
	}
	return nil
}

func (is *Service) indexedThrough() int64 {
	var height int64 = 1<<63 - 1
	for _, h := range is.indexed {
		height = min(height, h)
	}
	return height
}

func (is *Service) replay(ctx context.Context, through int64) error {
	r, ok := is.recovery.Get()
	if !ok {
		return nil
	}
	state, err := r.StateStore.Load()
	if err != nil {
		return err
	}
	if through > state.LastBlockHeight {
		return fmt.Errorf("replay height %d exceeds committed state height %d", through, state.LastBlockHeight)
	}
	if through > r.BlockStore.Height() {
		return fmt.Errorf("committed replay height %d exceeds stored blocks", through)
	}
	for height := is.indexedThrough() + 1; height <= through; height++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if height < r.BlockStore.Base() {
			return fmt.Errorf("unindexed block %d has been pruned", height)
		}
		block := r.BlockStore.LoadBlock(height)
		if block == nil || block.Height != height {
			return fmt.Errorf("missing replay block %d", height)
		}
		response, err := r.StateStore.LoadFinalizeBlockResponses(height)
		if err != nil {
			return fmt.Errorf("loading replay responses at %d: %w", height, err)
		}
		if response == nil || len(response.TxResults) != len(block.Txs) {
			return fmt.Errorf("replay transaction/result count mismatch at %d", height)
		}
		batch := NewBatch(int64(len(block.Txs)))
		for i, tx := range block.Txs {
			if response.TxResults[i] == nil {
				return fmt.Errorf("missing replay transaction result at %d/%d", height, i)
			}
			if err := batch.Add(&abci.TxResultV2{
				Height: height, Index: uint32(i), Tx: tx, Result: *response.TxResults[i],
			}); err != nil {
				return err
			}
		}
		if err := is.indexBatch(ctx, types.EventDataNewBlockHeader{
			Header: block.Header, NumTxs: int64(len(block.Txs)), ResultFinalizeBlock: *response,
		}, batch); err != nil {
			return err
		}
	}
	return nil
}
