package internaltest

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestInfrastructureFixturesProvideIsolatedDatabaseAndRedis(t *testing.T) {
	dsn, pool := NewPostgres(t)
	if dsn == "" {
		t.Fatal("database fixture returned no DSN")
	}
	var schema string
	if err := pool.QueryRow(context.Background(), `SELECT current_schema()`).Scan(&schema); err != nil || schema == "" {
		t.Fatalf("database fixture query: %q, %v", schema, err)
	}
	redisURL := NewRedisURL(t)
	options, err := goredis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(options)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := client.Set(context.Background(), "fixture-key", "stored", 0).Err(); err != nil {
		t.Fatal(err)
	}
	value, err := client.Get(context.Background(), "fixture-key").Result()
	if err != nil || value != "stored" {
		t.Fatalf("redis fixture value: %q, %v", value, err)
	}
}

func TestObjectStorePreservesArchiveBytes(t *testing.T) {
	objects := NewObjectStore(t)
	const key = "archive/segment.json"
	const body = `{"formatVersion":"raw-replay-v2"}`
	if err := objects.Put(context.Background(), key, strings.NewReader(body), int64(len(body)), "application/json"); err != nil {
		t.Fatal(err)
	}
	object, err := objects.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	read, err := io.ReadAll(object.Body)
	if err != nil || string(read) != body || object.Size != int64(len(body)) {
		t.Fatalf("stored archive bytes: %q, size=%d, err=%v", read, object.Size, err)
	}
}

func TestChainExposesCommittedCanonicalBlock(t *testing.T) {
	rpc, block := NewChain(t)
	chainID, err := rpc.ChainID(context.Background())
	if err != nil || chainID.Int64() != 125 || block.Height != 1 || block.Hash == "" || block.Time <= 0 {
		t.Fatalf("committed chain block: chain=%v block=%+v err=%v", chainID, block, err)
	}
}

func TestChainAtPreservesHistoricalBlockTime(t *testing.T) {
	_, block := NewChainAt(t, 1577836800)
	if block.Time < 1577836800 || block.Time > 1577836900 {
		t.Fatalf("historical block time: %d", block.Time)
	}
}

func TestBrokerPersistsPublishedSubject(t *testing.T) {
	connection, stream := NewBroker(t, "CHART_TEST", []string{"chart.test"})
	if err := connection.Publish("chart.test", []byte("committed")); err != nil {
		t.Fatal(err)
	}
	if err := connection.Flush(); err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(context.Background())
	if err != nil || info.State.Msgs != 1 {
		t.Fatalf("file-backed broker message: %+v, %v", info, err)
	}
}

func TestMarketChainCommitsCreationAndOrderedSwaps(t *testing.T) {
	chain := NewMarketChain(t)
	if chain.Creation == nil || len(chain.Swaps) != 2 || chain.Creation.BlockNumber.Int64() >= chain.Swaps[0].BlockNumber.Int64() || chain.Swaps[0].BlockNumber.Int64() >= chain.Swaps[1].BlockNumber.Int64() || len(chain.Headers) < 3 || chain.Pool == chain.Token {
		t.Fatalf("canonical market events: creation=%+v swaps=%d headers=%d", chain.Creation, len(chain.Swaps), len(chain.Headers))
	}
}

func TestMarketChainPreservesHistoricalIdleSpan(t *testing.T) {
	const timestamp = uint64(1703980800)
	chain := NewMarketChainAt(t, timestamp, 9*24*time.Hour)
	creation := chain.Headers[chain.Creation.BlockNumber.Int64()]
	firstSwap := chain.Headers[chain.Swaps[0].BlockNumber.Int64()]
	if creation.Time < int64(timestamp)+int64((9*24*time.Hour)/time.Second) || firstSwap.Time <= creation.Time {
		t.Fatalf("historical market order: creation=%d firstSwap=%d", creation.Time, firstSwap.Time)
	}
	if chain.Headers[chain.Swaps[1].BlockNumber.Int64()].Time <= firstSwap.Time {
		t.Fatalf("second market swap did not follow first: %+v", chain.Headers)
	}
}
