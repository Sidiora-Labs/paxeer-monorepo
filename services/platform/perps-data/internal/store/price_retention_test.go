package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func retentionRegistry(t *testing.T) *decode.Registry {
	t.Helper()
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{
		PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func retentionEvent(market string, skipped bool) abci.Event {
	event := abci.Event{Type: decode.EventAggregate}
	attrs := map[string]string{"market_id": market, "sources": "3", "required": "2"}
	if skipped {
		event.Type = decode.EventAggregateSkipped
	} else {
		for k, v := range map[string]string{"index_price": "50000000000000000000000", "mark_price": "50000000000000000000000", "basis": "0", "timestamp_ms": "1700000000000", "height": "1"} {
			attrs[k] = v
		}
	}
	for k, v := range attrs {
		event.Attributes = append(event.Attributes, abci.EventAttribute{Key: []byte(k), Value: []byte(v)})
	}
	return event
}

func retentionWrite(t *testing.T, db *sql.DB, registry *decode.Registry, height uint64, at time.Time, events ...abci.Event) {
	t.Helper()
	ctx := context.Background()
	b := header(height, at)
	input := decode.BlockInput{Number: height, Hash: b.Hash, Time: at, Events: events}
	if height == 1 {
		sender := decode.ModuleAddress("pricefeed")
		input.Logs = []decode.Log{{Log: ethtypes.Log{Address: decode.PricefeedAddress, BlockNumber: height, BlockHash: b.Hash,
			TxHash: crypto.Keccak256Hash([]byte("feed revocation")), TxIndex: 0, Index: 0,
			Topics: []common.Hash{crypto.Keccak256Hash([]byte("FeedKeyRevoked(address,address)")), common.HexToHash("01"), common.HexToHash("02")}},
			ReceiptKind: decode.ReceiptTx, TxFrom: &sender, BlockTime: at}}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	rows, err := registry.DecodeBlock(ctx, tx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Decoded) != len(events)+len(input.Logs) {
		t.Fatalf("decoded %d rows", len(rows.Decoded))
	}
	if _, err = WriteFacts(ctx, tx, b, rows); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func retentionExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func retentionLease(t *testing.T, db *sql.DB) {
	t.Helper()
	retentionExec(t, db, `INSERT INTO perps.writer_lease(name,holder,expires_at)
        VALUES('indexer','writer',clock_timestamp()+interval '10 minutes')`)
}

func retentionKeys(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT block::text || ':' || market_id::text || ':' || kind FROM perps.index_prices ORDER BY block, market_id, log_index`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}

func retentionOtherTables(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	tables, err := db.Query(`SELECT tablename FROM pg_tables WHERE schemaname = 'perps'
        AND tablename NOT IN ('index_prices','protocol_events') ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := tables.Err(); err != nil {
		t.Fatal(err)
	}
	if err := tables.Close(); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, name := range names {
		var contents string
		if err := db.QueryRow(`SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text,'[]') FROM perps.` + name + ` t`).Scan(&contents); err != nil {
			t.Fatal(err)
		}
		out[name] = contents
	}
	return out
}

func TestPriceRetentionExactRows(t *testing.T) {
	db := testDB(t)
	migrate(t, db)
	registry := retentionRegistry(t)
	old := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	cutoff := old.AddDate(0, 0, 7).Add(30 * time.Second)
	retentionWrite(t, db, registry, 1, old.Add(time.Second), retentionEvent("0", false), retentionEvent("1", false), retentionEvent("2", true), retentionEvent("4", true))
	retentionWrite(t, db, registry, 2, old.Add(30*time.Second), retentionEvent("0", false), retentionEvent("1", false), retentionEvent("2", true), retentionEvent("4", true))
	retentionWrite(t, db, registry, 3, old.Add(59*time.Second), retentionEvent("0", false), retentionEvent("1", true))
	retentionWrite(t, db, registry, 4, old.Add(time.Minute), retentionEvent("0", false))
	retentionWrite(t, db, registry, 5, cutoff.Add(-time.Microsecond), retentionEvent("3", false), retentionEvent("0", true))
	retentionWrite(t, db, registry, 6, cutoff, retentionEvent("3", false), retentionEvent("0", false), retentionEvent("2", true))
	retentionWrite(t, db, registry, 7, cutoff.Add(time.Second), retentionEvent("0", false))
	retentionLease(t, db)
	before := retentionOtherTables(t, db)
	p := &PriceRetention{}
	if _, err := p.Pass(context.Background(), db, cutoff, "indexer", "other"); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("nonwriter: %v", err)
	}
	if got := len(retentionKeys(t, db)); got != 17 {
		t.Fatalf("nonwriter leaves %d rows, want 17", got)
	}
	result, err := p.Pass(context.Background(), db, cutoff, "indexer", "writer")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2:1:aggregate", "2:4:skipped", "3:0:aggregate", "3:1:skipped", "4:0:aggregate", "5:3:aggregate", "6:0:aggregate", "6:2:skipped", "6:3:aggregate", "7:0:aggregate"}
	if got := retentionKeys(t, db); !slices.Equal(got, want) {
		t.Fatalf("surviving observations = %v, want %v", got, want)
	}
	if result.PriceRows != 7 || result.EventRows != 13 {
		t.Fatalf("removed %+v, want 7 prices and 13 events", result)
	}
	var events string
	if err := db.QueryRow(`SELECT string_agg(block::text || ':' || event_name, ',' ORDER BY block, log_index) FROM perps.protocol_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if want := "1:FeedKeyRevoked,6:pricefeed_aggregate,6:pricefeed_aggregate,6:pricefeed_aggregate_skipped,7:pricefeed_aggregate"; events != want {
		t.Fatalf("remaining events %s, want %s", events, want)
	}
	if after := retentionOtherTables(t, db); !reflect.DeepEqual(before, after) {
		t.Fatal("retention changed another table")
	}
	result, err = p.Pass(context.Background(), db, cutoff, "indexer", "writer")
	if err != nil || result != (PriceRetentionResult{}) {
		t.Fatalf("second pass %+v: %v", result, err)
	}
	// Recovery reads accept the sparse surviving facts in source order.
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := loadFacts(context.Background(), tx, 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, rows := range facts {
		for _, row := range rows {
			if row.Table() == "index_prices" {
				found++
			}
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if found != len(want) {
		t.Fatalf("recovery reads %d prices, want %d", found, len(want))
	}
	// Writing an old block again restores its original grain; its surviving
	// observation ids and the rows of other tables stay stable.
	retentionWrite(t, db, registry, 1, old.Add(time.Second), retentionEvent("0", false), retentionEvent("1", false), retentionEvent("2", true), retentionEvent("4", true))
	if got := len(retentionKeys(t, db)); got != len(want)+4 {
		t.Fatalf("replayed observations = %d", got)
	}
	result, err = p.Pass(context.Background(), db, cutoff, "indexer", "writer")
	if err != nil || result.PriceRows != 4 || result.EventRows != 4 {
		t.Fatalf("replayed retention %+v: %v", result, err)
	}
	if got := retentionKeys(t, db); !slices.Equal(got, want) {
		t.Fatalf("replayed survivors %v", got)
	}
	if after := retentionOtherTables(t, db); !reflect.DeepEqual(before, after) {
		t.Fatal("replay and retention changed other stored rows")
	}
}

// retentionBulk writes full-grain database rows with unique source positions.
func retentionBulk(t *testing.T, db *sql.DB, n int, at time.Time, minuteSamples bool) {
	t.Helper()
	spacing := "1 second"
	if minuteSamples {
		spacing = "1 minute"
	}
	retentionExec(t, db, `INSERT INTO perps.index_prices
        (block,block_hash,tx_index,log_index,sub,tx_hash,receipt_kind,block_time,emitter,kind,account,market_id,index_price,mark_price,basis,timestamp_ms,sources,required)
        SELECT g,decode(lpad(to_hex(g),64,'0'),'hex'),-1,0,0,decode(lpad(to_hex(g),64,'0'),'hex'),'block_event',
        $1::timestamptz + g * interval '`+spacing+`',decode(repeat('00',20),'hex'),'aggregate',decode(repeat('00',20),'hex'),
        0,g,g,0,1700000000000,3,2 FROM generate_series(1,$2::int) g`, at, n)
	retentionExec(t, db, `INSERT INTO perps.protocol_events
        (block,block_hash,tx_index,log_index,sub,tx_hash,receipt_kind,block_time,emitter,kind,account,event_name,market_id,args)
        SELECT block,block_hash,tx_index,log_index,sub,tx_hash,receipt_kind,block_time,emitter,kind,account,
        'pricefeed_aggregate',market_id,'{}'::jsonb FROM perps.index_prices`)
}

func retentionAudit(t *testing.T, db *sql.DB) {
	t.Helper()
	retentionExec(t, db, `CREATE TABLE public.price_retention_counts (table_name text, removed bigint);
        CREATE FUNCTION public.price_retention_count() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN INSERT INTO public.price_retention_counts SELECT TG_TABLE_NAME,count(*) FROM deleted; RETURN NULL; END $$;
        CREATE TRIGGER price_retention_count AFTER DELETE ON perps.index_prices REFERENCING OLD TABLE AS deleted
            FOR EACH STATEMENT EXECUTE FUNCTION public.price_retention_count();
        CREATE TRIGGER price_retention_count AFTER DELETE ON perps.protocol_events REFERENCING OLD TABLE AS deleted
            FOR EACH STATEMENT EXECUTE FUNCTION public.price_retention_count()`)
}

func TestPriceRetentionBoundedContinuation(t *testing.T) {
	db := testDB(t)
	migrate(t, db)
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	n := PriceRetentionBatch*PriceRetentionBatches + 5
	// All earlier candidates are already minute samples. Work must continue
	// past those samples to reach the redundant tail.
	retentionBulk(t, db, n, at, true)
	retentionExec(t, db, `INSERT INTO perps.index_prices
        (block,block_hash,tx_index,log_index,sub,tx_hash,receipt_kind,block_time,emitter,kind,account,market_id,index_price,mark_price,basis,timestamp_ms,sources,required)
        SELECT block+100000,decode(repeat('ab',32),'hex'),tx_index,log_index,sub,decode(repeat('ab',32),'hex'),receipt_kind,
        block_time+interval '1 second',emitter,kind,account,market_id,index_price,mark_price,basis,timestamp_ms,sources,required
        FROM perps.index_prices ORDER BY block DESC LIMIT 1`)
	retentionLease(t, db)
	retentionAudit(t, db)
	cutoff := at.AddDate(2, 0, 0)
	p := &PriceRetention{}
	first, err := p.Pass(context.Background(), db, cutoff, "indexer", "writer")
	if err != nil || first.PriceRows != 0 || first.EventRows != int64(PriceRetentionBatch*PriceRetentionBatches) {
		t.Fatalf("first pass %+v: %v", first, err)
	}
	cursorAt, cursorID := p.afterTime, p.afterID
	if cursorAt.IsZero() || cursorID == 0 {
		t.Fatal("full pass discarded its cursor")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Pass(cancelled, db, cutoff, "indexer", "writer"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled pass: %v", err)
	}
	if p.afterTime != cursorAt || p.afterID != cursorID {
		t.Fatal("cancelled statement advanced cursor")
	}
	second, err := p.Pass(context.Background(), db, cutoff, "indexer", "writer")
	if err != nil || second.PriceRows != 1 || second.EventRows != 5 {
		t.Fatalf("continued pass %+v: %v", second, err)
	}
	// An insertion behind the cursor is visited after the sweep wraps.
	retentionExec(t, db, `INSERT INTO perps.index_prices
        (block,block_hash,tx_index,log_index,sub,tx_hash,receipt_kind,block_time,emitter,kind,account,market_id,sources,required)
        VALUES(0,decode(repeat('cd',32),'hex'),-1,0,0,decode(repeat('cd',32),'hex'),'block_event',$1,
        decode(repeat('00',20),'hex'),'skipped',decode(repeat('00',20),'hex'),0,1,2)`, at)
	third, err := p.Pass(context.Background(), db, cutoff, "indexer", "writer")
	if err != nil || third.PriceRows != 1 || third.EventRows != 0 {
		t.Fatalf("wrapped pass %+v: %v", third, err)
	}
	for range 2 {
		result, err := p.Pass(context.Background(), db, cutoff, "indexer", "writer")
		if err != nil || result != (PriceRetentionResult{}) {
			t.Fatalf("idempotent sweep %+v: %v", result, err)
		}
	}
	var maxRows int64
	if err := db.QueryRow(`SELECT max(removed) FROM public.price_retention_counts`).Scan(&maxRows); err != nil || maxRows != PriceRetentionBatch {
		t.Fatalf("largest statement deletes %d: %v", maxRows, err)
	}
	var total int64
	if err := db.QueryRow(`SELECT count(*) FROM perps.index_prices`).Scan(&total); err != nil || total != int64(n) {
		t.Fatalf("sample rows %d: %v", total, err)
	}
}

func TestPriceRetentionIndexedPlans(t *testing.T) {
	db := testDB(t)
	migrate(t, db)
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	retentionBulk(t, db, 200000, at, false)
	retentionLease(t, db)
	retentionAudit(t, db)
	retentionExec(t, db, `ANALYZE perps.index_prices; ANALYZE perps.protocol_events`)
	cutoff := at.AddDate(0, 0, 7)
	queries := []struct {
		name, sql string
		args      []any
	}{
		{"prices", retainPricesSQL, []any{cutoff, time.Time{}, int64(0), "indexer", "writer"}},
		{"events", retainPriceEventsSQL, []any{cutoff, "indexer", "writer"}},
		{"prices_cursor", retainPricesSQL, []any{cutoff, at.Add(100000 * time.Second), int64(100000), "indexer", "writer"}},
	}
	for _, q := range queries {
		t.Run(q.name, func(t *testing.T) {
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			var raw []byte
			if err := tx.QueryRow(`EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) `+q.sql, q.args...).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var plan any
			if err := json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			scans := 0
			var visit func(any)
			visit = func(value any) {
				switch node := value.(type) {
				case []any:
					for _, child := range node {
						visit(child)
					}
				case map[string]any:
					if relation, ok := node["Relation Name"].(string); ok && (relation == "index_prices" || relation == "protocol_events") {
						typ, _ := node["Node Type"].(string)
						if strings.Contains(typ, "Seq Scan") {
							t.Fatalf("sequential fact read: %s", raw)
						}
						if strings.Contains(typ, "Index") || strings.Contains(typ, "Bitmap") {
							scans++
						}
					}
					for _, child := range node {
						visit(child)
					}
				}
			}
			visit(plan)
			if scans < 2 {
				t.Fatalf("missing candidate or deletion index: %s", raw)
			}
			var maxRows int64
			if err := tx.QueryRow(`SELECT max(removed) FROM public.price_retention_counts`).Scan(&maxRows); err != nil || maxRows > PriceRetentionBatch {
				t.Fatalf("statement rows %d: %v", maxRows, err)
			}
			t.Logf("%s: indexed fact reads=%s, deleted=%s", q.name, strconv.Itoa(scans), fmt.Sprint(maxRows))
		})
	}
}

func TestPriceRetentionMinuteAndSourceOrder(t *testing.T) {
	db := testDB(t)
	migrate(t, db)
	registry := retentionRegistry(t)
	zone := time.FixedZone("price time", 3600)
	before := time.Date(2026, 10, 1, 13, 0, 59, 0, zone)
	retentionWrite(t, db, registry, 1, before, retentionEvent("0", false), retentionEvent("0", false), retentionEvent("0", true))
	retentionWrite(t, db, registry, 2, before.Add(time.Second), retentionEvent("0", false), retentionEvent("0", false), retentionEvent("1", false))
	retentionLease(t, db)
	p := &PriceRetention{}
	result, err := p.Pass(context.Background(), db, before.AddDate(0, 0, 7), "indexer", "writer")
	if err != nil || result.PriceRows != 3 || result.EventRows != 6 {
		t.Fatalf("minute retention %+v: %v", result, err)
	}
	rows, err := db.Query(`SELECT block,log_index,market_id::int FROM perps.index_prices ORDER BY block,log_index`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keys [][3]int
	for rows.Next() {
		var key [3]int
		if err := rows.Scan(&key[0], &key[1], &key[2]); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := [][3]int{{1, 1, 0}, {2, 1, 0}, {2, 2, 1}}; !slices.Equal(keys, want) {
		t.Fatalf("minute source keys %v, want %v", keys, want)
	}
}
