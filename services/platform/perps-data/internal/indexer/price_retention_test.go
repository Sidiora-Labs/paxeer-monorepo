package indexer

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
	"github.com/ethereum/go-ethereum/common"
)

func TestPriceRetentionWriterMaintenance(t *testing.T) {
	db := boundedIndexerDB(t)
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed")})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Minute)
	old := now.AddDate(0, 0, -3)
	// Two days of full grain keep the recent pair while reducing the old pair.
	for i, at := range []time.Time{old.Add(time.Second), old.Add(2 * time.Second), now.Add(time.Second), now.Add(2 * time.Second)} {
		height := uint64(i + 1)
		hash := common.Hash{31: byte(height)}
		event := abci.Event{Type: decode.EventAggregate}
		for k, v := range map[string]string{"market_id": "0", "sources": "3", "required": "2", "index_price": "100", "mark_price": "100", "basis": "0", "timestamp_ms": "1700000000000", "height": "1"} {
			event.Attributes = append(event.Attributes, abci.EventAttribute{Key: []byte(k), Value: []byte(v)})
		}
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := registry.DecodeBlock(context.Background(), tx, decode.BlockInput{Number: height, Hash: hash, Time: at, Events: []abci.Event{event}})
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if _, err := store.WriteFacts(context.Background(), tx, store.Block{Number: height, Hash: hash, Time: at}, rows); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	ix := &Indexer{db: db, holder: "writer", logger: slog.Default(), priceRetentionDays: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ix.prunePrices(ctx); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("nonwriter maintenance: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM perps.index_prices`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("nonwriter leaves %d prices: %v", count, err)
	}
	if _, err := db.Exec(`INSERT INTO perps.writer_lease(name,holder,expires_at) VALUES('indexer','writer',clock_timestamp()+interval '1 minute')`); err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- ix.prunePrices(runCtx) }()
	defer func() {
		stop()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("maintenance stop: %v", err)
		}
	}()
	for {
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM perps.protocol_events`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	if err := db.QueryRow(`SELECT count(*) FROM perps.index_prices`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("writer leaves %d prices: %v", count, err)
	}
	// More old rows wait for the next scheduled pass.
	if _, err := db.Exec(`INSERT INTO perps.protocol_events
        (block,block_hash,tx_index,log_index,sub,tx_hash,receipt_kind,block_time,emitter,kind,account,event_name,market_id,args)
        SELECT 10,decode(repeat('ab',32),'hex'),tx_index,log_index,sub,decode(repeat('ab',32),'hex'),receipt_kind,
        $1,emitter,kind,account,event_name,market_id,args FROM perps.protocol_events ORDER BY block LIMIT 1`, old); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-timer.C:
	}
	if err := db.QueryRow(`SELECT count(*) FROM perps.protocol_events`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("maintenance interval leaves %d events: %v", count, err)
	}
}
