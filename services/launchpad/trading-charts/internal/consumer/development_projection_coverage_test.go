package consumer

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
)

func TestDevelopmentSourceProjectsCommittedSwap(t *testing.T) {
	ctx := context.Background()
	_, pool := internaltest.NewPostgres(t)
	rpc, row := repair314Source(t)
	opt, err := redis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	chartStore := store.New(pool)
	builder := engine.New(pool, rdb, chartStore, logger)
	source := NewDevelopmentSource(pool, rpc, builder, logger)
	if _, err = pool.Exec(ctx, `CREATE SCHEMA indexer;
CREATE TABLE indexer.cursors(chain_id bigint PRIMARY KEY,last_processed_block bigint NOT NULL);
CREATE TABLE indexer.swaps(pool_id text,pool_address text,sender text,is_buy boolean,amount_in text,amount_out text,fee text,price text,block_number bigint,block_timestamp bigint,tx_hash text,log_index bigint)`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.cursors VALUES(125,$1)`, row.height); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.swaps VALUES($2,$3,$4,$5,$6,$7,$8,$9,$1,$10,$11,$12)`, row.height, row.poolID, row.pool, row.sender, row.buy, row.amountIn, row.amountOut, row.fee, row.price, row.timestamp, row.hash, row.logIndex); err != nil {
		t.Fatal(err)
	}
	count, err := source.Project(ctx)
	if err != nil || count != 1 {
		t.Fatalf("projected source swaps: %d, %v", count, err)
	}
	candle, err := chartStore.GetCandle(ctx, row.pool, "1m", row.timestamp/60*60)
	if err != nil || candle == nil || candle.TradeCount != 1 || candle.VolumeUsdl != row.amountIn {
		t.Fatalf("projected candle: %+v, %v", candle, err)
	}
	count, err = source.Project(ctx)
	if err != nil || count != 0 {
		t.Fatalf("replayed committed source: %d, %v", count, err)
	}
	missingRPC := NewDevelopmentSource(pool, nil, builder, logger)
	if count, err := missingRPC.Project(ctx); count != 0 || err == nil || !strings.Contains(err.Error(), "development source RPC required") {
		t.Fatalf("source without canonical RPC: %d, %v", count, err)
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE indexer.swaps RENAME TO swaps_held`); err != nil {
		t.Fatal(err)
	}
	if count, err := source.Project(ctx); count != 0 || err == nil || !strings.Contains(err.Error(), "indexer.swaps") {
		t.Fatalf("unavailable source relation: %d, %v", count, err)
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE indexer.swaps_held RENAME TO swaps`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE candles.development_state RENAME TO development_state_held`); err != nil {
		t.Fatal(err)
	}
	if count, err := source.Project(ctx); count != 0 || err == nil || !strings.Contains(err.Error(), "development_state") {
		t.Fatalf("unavailable relay relation: %d, %v", count, err)
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE candles.development_state_held RENAME TO development_state`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.development_state(key,payload) VALUES('outbox/not-json',convert_to('[1]','UTF8'))`); err != nil {
		t.Fatal(err)
	}
	if count, err = source.Project(ctx); err == nil || count != 0 || !strings.Contains(err.Error(), "invalid development outbox key") {
		t.Fatalf("invalid relay key: %d, %v", count, err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM candles.development_state WHERE key='outbox/not-json'`); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { source.Run(runCtx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("development source did not stop after cancellation")
	}
	address := "0x" + strings.Repeat("2", 40)
	word := common.BytesToHash(common.HexToAddress(address).Bytes())
	if !logNamesAddress([]string{word.Hex()}, nil, address) || !logNamesAddress(nil, word.Bytes(), address) || logNamesAddress(nil, nil, address) {
		t.Fatal("pool address word matching")
	}
	consumer, err := New(builder, internaltest.NewRedisURL(t), logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = consumer.Close() })
	consumer.SetDevelopmentSource(source)
	if consumer.source != source {
		t.Fatal("development source was not bound to swap consumer")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.swaps SELECT NULL,pool_address,sender,is_buy,amount_in,amount_out,fee,price,block_number,block_timestamp,tx_hash,log_index+1 FROM indexer.swaps LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if count, err := source.Project(ctx); count != 0 || err == nil || !strings.Contains(err.Error(), "cannot scan NULL") {
		t.Fatalf("incomplete indexed row: %d, %v", count, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM indexer.swaps WHERE pool_id IS NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.swaps SELECT pool_id,pool_address,sender,is_buy,amount_in,amount_out,fee,price,block_number,block_timestamp,tx_hash,log_index+1 FROM indexer.swaps LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if count, err := source.Project(ctx); count != 0 || err == nil || !strings.Contains(err.Error(), "Swap absent from receipt") {
		t.Fatalf("unbound indexed position: %d, %v", count, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM indexer.swaps WHERE log_index=$1`, row.logIndex+1); err != nil {
		t.Fatal(err)
	}
	poolKey, err := json.Marshal([]string{row.pool})
	if err != nil {
		t.Fatal(err)
	}
	key := "outbox/" + string(poolKey)
	var original []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM candles.development_state WHERE key=$1`, key).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candles.development_state SET payload=$2 WHERE key=$1`, key, []byte(`invalid`)); err != nil {
		t.Fatal(err)
	}
	if count, err := source.Project(ctx); count != 0 || err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unreadable retained delivery: %d, %v", count, err)
	}
	var retained []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM candles.development_state WHERE key=$1`, key).Scan(&retained); err != nil || string(retained) != "invalid" {
		t.Fatalf("retained delivery after refusal: %q, %v", retained, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candles.development_state SET payload=$2 WHERE key=$1`, key, original); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM candles.processed_swaps WHERE tx_hash=$1 AND log_index=$2`, row.hash, row.logIndex); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE candles.candles RENAME TO candles_held`); err != nil {
		t.Fatal(err)
	}
	count, err = source.Project(ctx)
	if _, restoreErr := pool.Exec(ctx, `ALTER TABLE candles.candles_held RENAME TO candles`); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	if count != 0 || err == nil || !strings.Contains(err.Error(), "candles") {
		t.Fatalf("unavailable candle store: %d, %v", count, err)
	}
}

func TestDevelopmentSourceBindsIndexedRowsToCanonicalReceipt(t *testing.T) {
	ctx := context.Background()
	rpc, canonical := repair314Source(t)
	source := NewDevelopmentSource(nil, rpc, nil, slog.Default())
	frame, err := source.frame(ctx, canonical)
	if err != nil || frame.TxHash != canonical.hash || frame.LogIndex == nil || *frame.LogIndex != uint64(canonical.logIndex) {
		t.Fatalf("canonical indexed Swap frame: %+v, %v", frame, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*developmentSourceRow)
		want   string
	}{
		{"missing transaction", func(r *developmentSourceRow) { r.hash = "0x" + strings.Repeat("0", 64) }, "receipt position unavailable"},
		{"different height", func(r *developmentSourceRow) { r.height++ }, "receipt position unavailable"},
		{"different block time", func(r *developmentSourceRow) { r.timestamp++ }, "block authority mismatch"},
		{"different log", func(r *developmentSourceRow) { r.logIndex++ }, "Swap absent"},
		{"different sender", func(r *developmentSourceRow) { r.sender = "0x" + strings.Repeat("3", 40) }, "not the indexed Swap"},
		{"different amount", func(r *developmentSourceRow) { r.amountIn = "1000001" }, "Swap differs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := canonical
			tc.change(&row)
			if got, err := source.frame(ctx, row); err == nil || !strings.Contains(err.Error(), tc.want) || got.EventName != "" {
				t.Fatalf("unbound indexed row: %+v, %v", got, err)
			}
		})
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if got, err := source.frame(cancelled, canonical); err == nil || got.EventName != "" {
		t.Fatalf("cancelled canonical RPC frame: %+v, %v", got, err)
	}
}
