package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMarketRecoveryBindsCompleteReadsToCanonicalBlockContent(t *testing.T) {
	c := newPauseChain(t)
	address, contract, created := c.create(t, "Recovery reads")
	paused := c.transact(t, contract, "pause")
	s := &recoveringQualifiedSource{client: c.client, history: &source.HistoricalSource{RPC: &source.CommittedSource{}}}
	from, to := created.BlockNumber.Uint64(), paused.BlockNumber.Uint64()
	reads, err := s.readMarketHeights(c.ctx, from, to, 2)
	if err != nil || len(reads) != 2 {
		t.Fatalf("complete market reads: %d, %v", len(reads), err)
	}
	addresses, blocks, err := s.validateMarketReads(c.ctx, from, to, reads)
	found := false
	for _, candidate := range addresses {
		found = found || candidate == address
	}
	if err != nil || !found || len(blocks) != 2 || blocks[from].Hash != created.BlockHash || blocks[to].ParentHash != created.BlockHash {
		t.Fatalf("canonical market reads: %v, %+v, %v", addresses, blocks, err)
	}
	if _, _, err := s.marketAddresses(c.ctx, from, to); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing read", "header number", "parent link", "transaction root", "transaction position"} {
		t.Run(name, func(t *testing.T) {
			changed := append([]marketHeightRead(nil), reads...)
			switch name {
			case "missing read":
				changed = changed[:1]
			case "header number":
				changed[0].header.Number = nil
			case "parent link":
				changed[1].header.ParentHash = common.Hash{}
			case "transaction root":
				changed[0].header.TxHash = common.Hash{}
				changed[0].hash = changed[0].header.Hash()
				changed[0].rawTxs = nil
			case "transaction position":
				changed[0].rawTxs = append([]json.RawMessage(nil), changed[0].rawTxs...)
				var transaction map[string]json.RawMessage
				if err := json.Unmarshal(changed[0].rawTxs[0], &transaction); err != nil {
					t.Fatal(err)
				}
				transaction["transactionIndex"] = json.RawMessage(`"0x1"`)
				encoded, err := json.Marshal(transaction)
				if err != nil {
					t.Fatal(err)
				}
				changed[0].rawTxs[0] = encoded
			}
			if _, _, err := s.validateMarketReads(c.ctx, from, to, changed); !errors.Is(err, receipt.ErrBinding) {
				t.Fatalf("inconsistent market reads admitted: %v", err)
			}
		})
	}
	canceled, cancel := context.WithCancel(c.ctx)
	cancel()
	if _, _, err := s.marketAddresses(canceled, from, to); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled market read: %v", err)
	}
	var absent *recoveringQualifiedSource
	if _, _, err := absent.FetchQualifiedBatch(c.ctx, from, to); !errors.Is(err, receipt.ErrQualification) {
		t.Fatalf("absent recovery source: %v", err)
	}
	if _, _, err := absent.marketAddresses(c.ctx, from, to); !errors.Is(err, receipt.ErrQualification) {
		t.Fatalf("absent market source: %v", err)
	}
	if _, err := recoverBatchMarkets(c.ctx, nil, nil, to); !errors.Is(err, receipt.ErrQualification) {
		t.Fatalf("absent market inventory: %v", err)
	}
	c.retain(t, paused)
	pool := c.observation.pool
	inventory := postgresMarketInventory{pool: pool}
	hints := make([]common.Address, marketRecoveryPageSize+1)
	for i := range hints {
		hints[i] = address
	}
	markets, err := recoverBatchMarkets(c.ctx, inventory, hints, to)
	if err != nil || len(markets) != 1 || markets[0].Pool != address {
		t.Fatalf("duplicate inventory hints: %+v, %v", markets, err)
	}
	token := markets[0].Token
	if _, err := pool.Exec(c.ctx, `UPDATE indexer.indexer_canonical_creations SET pool=token`); err != nil {
		t.Fatal(err)
	}
	if _, err := inventory.Page(c.ctx, []common.Address{token}, to, ""); err == nil || !strings.Contains(err.Error(), "persisted canonical market identity") {
		t.Fatalf("inconsistent persisted market: %v", err)
	}
	if _, err := pool.Exec(c.ctx, `DROP VIEW indexer.indexer_canonical_creations_v1; ALTER TABLE indexer.indexer_canonical_creations ALTER COLUMN transaction_index TYPE numeric USING transaction_index::numeric; UPDATE indexer.indexer_canonical_creations SET transaction_index=0.5`); err != nil {
		t.Fatal(err)
	}
	pool.Reset()
	if _, err := inventory.Page(c.ctx, []common.Address{token}, to, ""); err == nil || !strings.Contains(err.Error(), "scan") {
		t.Fatalf("invalid persisted position: %v", err)
	}
	if _, err := pool.Exec(c.ctx, `ALTER TABLE indexer.indexer_canonical_creations RENAME TO unavailable_creations`); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverBatchMarkets(c.ctx, inventory, []common.Address{token}, to); err == nil {
		t.Fatal("unavailable inventory admitted")
	} else {
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
			t.Fatalf("inventory database error: %v", err)
		}
	}
	reserves := &recoveringReserveReader{}
	if _, err := reserves.PoolReserves(c.ctx, address, created.BlockHash); !errors.Is(err, receipt.ErrQualification) {
		t.Fatalf("unavailable reserve inventory: %v", err)
	}
}
