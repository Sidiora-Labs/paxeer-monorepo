package l16

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestRecordedInventoryRejectsInconsistentIndexedSources(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name, sql string
		want      error
		receipts  bool
	}{
		{"watermark behind", `UPDATE indexer.indexer_ingest_watermarks SET typed_complete_height=1`, delivery.ErrGap, false},
		{"incomplete block", `UPDATE indexer.indexer_block_completeness SET fetch_complete=false`, delivery.ErrGap, false},
		{"raw count mismatch", `UPDATE indexer.indexer_block_completeness SET raw_count=raw_count+1`, delivery.ErrGap, false},
		{"wrong completeness hash", `UPDATE indexer.indexer_block_completeness SET block_hash='0x'||repeat('e',64)`, delivery.ErrIntegrity, false},
		{"expired source lease", `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 minute'`, delivery.ErrLease, false},
		{"raw emitter mismatch", `UPDATE indexer.indexer_raw_logs r SET emitter='0x'||repeat('e',40) WHERE EXISTS(SELECT 1 FROM indexer.indexer_token_transfers_v1 t WHERE t.transaction_hash=r.transaction_hash AND t.log_index=r.log_index)`, delivery.ErrIntegrity, false},
		{"raw topic type", `UPDATE indexer.indexer_raw_logs r SET topics='{}'::jsonb WHERE EXISTS(SELECT 1 FROM indexer.indexer_token_transfers_v1 t WHERE t.transaction_hash=r.transaction_hash AND t.log_index=r.log_index)`, nil, false},
		{"raw topic count", `UPDATE indexer.indexer_raw_logs r SET topics='[]'::jsonb WHERE EXISTS(SELECT 1 FROM indexer.indexer_token_transfers_v1 t WHERE t.transaction_hash=r.transaction_hash AND t.log_index=r.log_index)`, delivery.ErrIntegrity, false},
		{"raw data mismatch", `UPDATE indexer.indexer_raw_logs r SET data=decode('00','hex') WHERE EXISTS(SELECT 1 FROM indexer.indexer_token_transfers_v1 t WHERE t.transaction_hash=r.transaction_hash AND t.log_index=r.log_index)`, delivery.ErrIntegrity, false},
		{"fill invalid side", `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{side}','"invalid"')`, nil, true},
		{"fill digest mismatch", `UPDATE indexer.indexer_accounting_fills SET source_fact_digest=repeat('0',64)`, delivery.ErrIntegrity, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, pool := recordedRebuildService(t, ctx)
			var raw []byte
			if err := pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 ORDER BY log_index LIMIT 1`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			fill, err := accounting.DecodeFillV1(raw)
			if err != nil {
				t.Fatal(err)
			}
			tag, err := pool.Exec(ctx, tc.sql)
			if err != nil || tag.RowsAffected() == 0 {
				t.Fatal(tag, err)
			}
			for _, mode := range []string{"ordinary", "archive", "live"} {
				t.Run(mode, func(t *testing.T) {
					var err error
					switch mode {
					case "ordinary":
						_, err = service.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
					case "archive":
						_, err = service.RebuildArchive(ctx, realMarketCreator, realMarketToken, 33963733)
					case "live":
						tx, e := pool.Begin(ctx)
						if e != nil {
							t.Fatal(e)
						}
						defer inventoryRollback(tx)
						_, err = service.liveRebuildTx(ctx, tx, realMarketCreator, realMarketToken, 33963733)
					}
					if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
						t.Fatalf("error=%v want=%v", err, tc.want)
					}
				})
			}
			if tc.receipts {
				tx, e := pool.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				defer inventoryRollback(tx)
				if _, e = service.authority.liveReceiptSettlement(ctx, tx, fill); e == nil {
					t.Fatal("inconsistent receipt source accepted")
				}
				if _, e = service.authority.markPriceTx(ctx, tx, realMarketToken, 33963733, fill.Position.BlockHash); e == nil {
					t.Fatal("inconsistent mark source accepted")
				}
			}
			var count int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_active_v2`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
}

func TestRecordedReceiptRequiresCommittedFill(t *testing.T) {
	ctx := context.Background()
	service, pool := recordedRebuildService(t, ctx)
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 ORDER BY log_index LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	fill, err := accounting.DecodeFillV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	if _, err = tx.Exec(ctx, `DELETE FROM indexer.indexer_accounting_fills`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.authority.receiptSettlement(ctx, tx, fill); !errors.Is(err, ErrCostAllocationUnavailable) {
		t.Fatal(err)
	}
	if _, err = service.authority.liveReceiptSettlement(ctx, tx, fill); !errors.Is(err, ErrCostAllocationUnavailable) {
		t.Fatal(err)
	}
	if mark, err := service.authority.markPriceTx(ctx, tx, realMarketToken, 33963733, fill.Position.BlockHash); err != nil || mark != nil {
		t.Fatal(mark, err)
	}
	if _, err = ReadSourceFillTx(ctx, tx, fill.SourceFillID, accounting.Digest(strings.Repeat("a", 64))); !errors.Is(err, delivery.ErrGap) {
		t.Fatal(err)
	}
	if _, err = service.authority.receiptSettlement(ctx, tx, accounting.FillV1{}); err == nil {
		t.Fatal("invalid receipt fill accepted")
	}
	if _, err = service.authority.liveReceiptSettlement(ctx, tx, accounting.FillV1{}); err == nil {
		t.Fatal("invalid live fill accepted")
	}
}
