package rest_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestDevelopmentStreamProjectsIndexedPoolStats(t *testing.T) {
	f := newMktMarketFixture(t, true)
	ctx := context.Background()
	const address = "0x1111111111111111111111111111111111111111"
	const token = "0x2222222222222222222222222222222222222222"
	receipt := f.createPool(t, address, token, mktPoolOptical, 1760000000)
	f.insertState(t, address, receipt, "1000000", "2000000", "3000000")
	f.insertPoolStats(t, address, token, "1000000", "10", 1, 0, 1, 1760000000, 1760000001)
	f.setCursor(t, receipt.BlockNumber.Uint64())
	d, err := rest.NewDevelopmentStream(ctx, rest.DevelopmentStreamOptions{
		Pool: f.pool, Store: store.New(f.pool, f.rdb), Redis: f.rdb, ChainRPCURL: f.chain.url,
		SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{},
	})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := d.UnknownTopics(ctx, []string{"market.stats." + address, "market.stats.0x9999999999999999999999999999999999999999", "market.rankings.trending"})
	if err != nil || len(unknown) != 1 || unknown[0] != "market.stats.0x9999999999999999999999999999999999999999" {
		t.Fatalf("unknown topics: %v, %v", unknown, err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO api_stream_dev.stream_projection(topic,sequence) VALUES($1,0)`, "market.trades."+address); err != nil {
		t.Fatal(err)
	}
	if err := d.Project(ctx); err != nil {
		t.Fatal(err)
	}
	var frame []byte
	if err := f.pool.QueryRow(ctx, `SELECT frame FROM api_stream_dev.stream_projection WHERE topic=$1`, "market.stats."+address).Scan(&frame); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Topic   string `json:"topic"`
		Payload struct {
			Pool string `json:"poolAddress"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(frame, &event); err != nil || event.Topic != "market.stats."+address || event.Payload.Pool != address {
		t.Fatalf("stats frame: %+v, %v", event, err)
	}
	if err := d.Project(ctx); err != nil {
		t.Fatalf("repeat projection: %v", err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM api_stream_dev.stream_delivery_outbox`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unchanged stats appended %d frames: %v", count, err)
	}
	withoutRPC, err := rest.NewDevelopmentStream(ctx, rest.DevelopmentStreamOptions{
		Pool: f.pool, Store: store.New(f.pool, f.rdb), SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := withoutRPC.Project(ctx); err == nil || !strings.Contains(err.Error(), "NETWORK_RPC_URL") {
		t.Fatalf("unconfigured projection: %v", err)
	}
}
