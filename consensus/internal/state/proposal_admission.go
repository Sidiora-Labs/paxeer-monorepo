package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

var proposalAdmissionKey = []byte("sequencer/proposal-admission/v1")

type proposalAdmission struct {
	Pending      types.Txs
	Acknowledged map[string]bool
}

type proposalAdmissionStore interface {
	loadProposalAdmission() (proposalAdmission, error)
	saveProposalAdmission(proposalAdmission) error
}

type acknowledgingSequencer interface {
	AdmitWithAcknowledgements(context.Context, int64, types.Txs) (int, int, error)
}

func (store dbStore) loadProposalAdmission() (proposalAdmission, error) {
	journal := proposalAdmission{Acknowledged: make(map[string]bool)}
	data, err := store.db.Get(proposalAdmissionKey)
	if err != nil {
		return journal, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &journal); err != nil {
			return journal, err
		}
	}
	if journal.Acknowledged == nil {
		journal.Acknowledged = make(map[string]bool)
	}
	return journal, nil
}

func (store dbStore) saveProposalAdmission(journal proposalAdmission) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	return store.db.SetSync(proposalAdmissionKey, data)
}

func (blockExec *BlockExecutor) admitProposalTransactions(ctx context.Context, height int64, limits mempool.ReapLimits) error {
	source, ok := blockExec.sequencer.(acknowledgingSequencer)
	if !ok {
		return fmt.Errorf("sequencer source does not report admission acknowledgements")
	}
	store, ok := blockExec.store.(proposalAdmissionStore)
	if !ok {
		return fmt.Errorf("state store does not persist proposal admission")
	}
	// Keep the selected mempool snapshot stable through its acknowledgement.
	// Reaping a smaller count is unsafe: ReapTxs groups EVM transactions after
	// selecting by priority, so its returned prefix need not be a reap prefix.
	blockExec.mempool.Lock()
	defer blockExec.mempool.Unlock()
	journal, err := store.loadProposalAdmission()
	if err != nil {
		return fmt.Errorf("load proposal admission: %w", err)
	}
	ready, _ := blockExec.mempool.ReapTxs(mempool.ReapLimits{}, false)
	present := make(map[string]bool, len(ready))
	for _, tx := range ready {
		present[tx.Hash().String()] = true
	}
	for hash := range journal.Acknowledged {
		if !present[hash] {
			delete(journal.Acknowledged, hash)
		}
	}
	if len(journal.Pending) == 0 {
		reaped, _ := blockExec.mempool.ReapTxs(limits, false)
		for _, tx := range reaped {
			if !journal.Acknowledged[tx.Hash().String()] {
				journal.Pending = append(journal.Pending, tx)
			}
		}
	}
	// Persist the unprocessed work before it can leave the mempool, including
	// transactions arriving after a height was sealed. Reopening needs no peer
	// resubmission to recover this suffix.
	if err := store.saveProposalAdmission(journal); err != nil {
		return fmt.Errorf("save proposal admission: %w", err)
	}
	if len(journal.Pending) > 0 {
		admitted, processed, admitErr := source.AdmitWithAcknowledgements(ctx, height, journal.Pending)
		if processed < 0 || processed > len(journal.Pending) || admitted < 0 || admitted > processed {
			return fmt.Errorf("invalid admission acknowledgement: admitted %d, processed %d of %d", admitted, processed, len(journal.Pending))
		}
		for _, tx := range journal.Pending[:processed] {
			journal.Acknowledged[tx.Hash().String()] = true
		}
		journal.Pending = journal.Pending[processed:]
		if err := store.saveProposalAdmission(journal); err != nil {
			return fmt.Errorf("acknowledge proposal admission: %w", err)
		}
		logger.Info("admitted consensus mempool transactions", "height", height, "admitted", admitted, "processed", processed, "pending", len(journal.Pending))
		if admitErr != nil {
			return admitErr
		}
	}
	// There is no arbitrary-hash removal API on TxMempool. Drain only when
	// every ready transaction has an explicit outcome; otherwise keep the
	// acknowledged prefix too, excluding it from subsequent admission calls.
	for _, tx := range ready {
		if !journal.Acknowledged[tx.Hash().String()] {
			return nil
		}
	}
	blockExec.mempool.ReapTxs(mempool.ReapLimits{}, true)
	return nil
}
