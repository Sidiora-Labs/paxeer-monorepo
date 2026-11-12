package l16

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestRebuiltHistoryPreservesTransferQualityAndTransactionGas(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	base, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	at := base.ObservedThrough.Add(-24 * time.Hour)
	day := at.Truncate(24 * time.Hour)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
		t.Fatal(err)
	}
	tokens := []string{"0x" + strings.Repeat("a", 40), "0x" + strings.Repeat("b", 40)}
	for i, token := range tokens {
		state := Empty()
		records := []EconomicRecord{}
		last := ""
		decimals := 18
		for j, kind := range []string{movementIncoming, sideBuy, movementOutgoing, sideSell} {
			leg := testLeg(uint32(i*10 + j + 1))
			leg.Wallet = realMarketCreator
			leg.Token = token
			leg.Sender = realMarketCreator
			leg.Payer = realMarketCreator
			leg.Recipient = realMarketCreator
			leg.Height = 100
			leg.TransactionIndex = uint32(i)
			leg.Identity.Transaction = "0x" + strings.Repeat(string(rune('c'+i)), 64)
			leg.Predecessor = last
			leg.Kind = kind
			leg.Quantity = "10"
			leg.ActualUSDL = nil
			if kind == sideBuy {
				leg.ActualUSDL = str("100")
			}
			if kind == sideSell {
				leg.Quantity = "5"
				leg.ActualUSDL = str("20")
			}
			if kind == movementOutgoing {
				leg.Quantity = "5"
			}
			record := EconomicRecord{TimedLeg: TimedLeg{Leg: leg, CommittedAt: at}, BlockHash: base.BlockHash, SourceDigest: strings.Repeat("d", 64), GasPAXRaw: "7", TokenDecimals: &decimals}
			if kind == movementOutgoing || kind == sideSell {
				var allocation Disposal
				state, allocation, err = Dispose(state, leg.Quantity, leg.ActualUSDL)
				record.Allocation = &allocation
			} else {
				state, err = Apply(state, leg)
			}
			if err != nil {
				t.Fatal(err)
			}
			record.State = state
			records = append(records, record)
			last, err = leg.Identity.Key()
			if err != nil {
				t.Fatal(err)
			}
		}
		raw, err := json.Marshal(records)
		if err != nil {
			t.Fatal(err)
		}
		replay := base
		replay.Partition = "125:" + realMarketCreator + ":" + token
		replay.State = state
		replay.LastIdentity = last
		replay.InputDigest = delivery.Hash(raw)
		replay.Checkpoints = []Checkpoint{{Day: day.Format("2006-01-02"), CompleteHistory: true, State: state}}
		if err = s.persistReplayTx(ctx, tx, replay, records); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	summary, err := s.PublishRebuiltWallet(ctx, realMarketCreator, base.Height, l17.Repository{Pool: pool})
	if err != nil || summary.PositionCount != 3 {
		t.Fatal(summary, err)
	}
	var count, unknown, partial int
	if err = pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE payload->>'kind'='transfer_in' AND payload->>'basisQuality'='unknown'),count(*) FILTER(WHERE payload->>'basisQuality'='partial') FROM pnl.portfolio_history_rows WHERE snapshot_id=$1`, summary.SnapshotID).Scan(&count, &unknown, &partial); err != nil || count != 9 || unknown != 2 || partial != 4 {
		t.Fatal(count, unknown, partial, err)
	}
	var gas string
	if err = pool.QueryRow(ctx, `SELECT payload->>'gasPaxRaw' FROM pnl.inventory_l16_wallet_intervals_v3 WHERE snapshot_id=$1 AND interval='1d' AND bucket=$2`, summary.SnapshotID, day).Scan(&gas); err != nil || gas != "14" {
		t.Fatal(gas, err)
	}
}
