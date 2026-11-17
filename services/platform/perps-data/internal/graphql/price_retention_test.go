package graphql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/publish"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
	"github.com/ethereum/go-ethereum/common"
	"github.com/lib/pq"
	"github.com/nats-io/nats.go"
)

func retentionReceive[T any](t *testing.T, ctx context.Context, ch <-chan T) T {
	t.Helper()
	select {
	case value, open := <-ch:
		if !open {
			t.Fatal("price subscription ended")
		}
		return value
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var zero T
	return zero
}

func retentionDatabase(t *testing.T, ctx context.Context, raw string) *sql.DB {
	t.Helper()
	dsn := raw
	if strings.HasPrefix(raw, "postgres://") || strings.HasPrefix(raw, "postgresql://") {
		var err error
		dsn, err = pq.ParseURL(raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	admin, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := "graphql_prices_" + strings.ToLower(rand.Text())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+pq.QuoteIdentifier(name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE " + pq.QuoteIdentifier(name)); err != nil {
			t.Error(err)
		}
		if err := admin.Close(); err != nil {
			t.Error(err)
		}
	})
	db, err := store.Open(dsn + " dbname=" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func TestPriceRetentionQueriesAndSubscriptions(t *testing.T) {
	raw := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := retentionDatabase(t, ctx, raw)
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed")})
	if err != nil {
		t.Fatal(err)
	}
	write := func(height uint64, at time.Time) {
		t.Helper()
		event := abci.Event{Type: decode.EventAggregate}
		for k, v := range map[string]string{"market_id": "0", "sources": "3", "required": "2", "index_price": strconv.FormatUint(height*100, 10), "mark_price": "100", "basis": "0", "timestamp_ms": strconv.FormatInt(at.UnixMilli(), 10), "height": strconv.FormatUint(height, 10)} {
			event.Attributes = append(event.Attributes, abci.EventAttribute{Key: []byte(k), Value: []byte(v)})
		}
		hash := common.Hash{31: byte(height)}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		rows, err := registry.DecodeBlock(ctx, tx, decode.BlockInput{Number: height, Hash: hash, Time: at, Events: []abci.Event{event}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.WriteFacts(ctx, tx, store.Block{Number: height, Hash: hash, Time: at}, rows); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Truncate(time.Minute)
	old := now.AddDate(0, 0, -8)
	write(1, old.Add(time.Second))
	var deletedCursor int64
	if err := db.QueryRow(`SELECT row_id FROM perps.index_prices WHERE block=1`).Scan(&deletedCursor); err != nil {
		t.Fatal(err)
	}
	feed, err := NewFeed(125, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &Resolver{DB: db}
	market := int32(0)
	subscribe := func(parent context.Context) (context.CancelFunc, <-chan *subPriceUpdate, <-chan *subPriceUpdate, <-chan []*subLatestPrice) {
		t.Helper()
		subCtx, stop := context.WithCancel(withFeed(parent, feed))
		all, err := resolver.PriceUpdated(subCtx, struct{ MarketID *int32 }{})
		if err != nil {
			t.Fatal(err)
		}
		one, err := resolver.PriceUpdated(subCtx, struct{ MarketID *int32 }{&market})
		if err != nil {
			t.Fatal(err)
		}
		latest, err := resolver.LatestPricesUpdated(subCtx)
		if err != nil {
			t.Fatal(err)
		}
		return stop, all, one, latest
	}
	stop, all, one, latest := subscribe(ctx)
	defer func() { stop() }()
	write(2, old.Add(2*time.Second))
	write(3, old.Add(3*time.Second))
	if _, err := db.Exec(`INSERT INTO perps.writer_lease(name,holder,expires_at) VALUES('indexer','writer',clock_timestamp()+interval '2 minutes')`); err != nil {
		t.Fatal(err)
	}
	retention := &store.PriceRetention{}
	result, err := retention.Pass(ctx, db, now.AddDate(0, 0, -7), "indexer", "writer")
	if err != nil || result.PriceRows != 2 || result.EventRows != 3 {
		t.Fatalf("retention %+v: %v", result, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM perps.index_prices WHERE row_id=$1`, deletedCursor).Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted cursor still has %d rows: %v", count, err)
	}
	schema, err := NewSchema(db)
	if err != nil {
		t.Fatal(err)
	}
	response := schema.Exec(ctx, `{ latestPrices { marketId price blockNumber } priceHistory(marketId:0) { blockNumber price }
        protocolEvents { blockNumber } market(marketId:0) { latestPrice { blockNumber } poolState { blockNumber } } }`, "", nil)
	if len(response.Errors) != 0 {
		t.Fatalf("query errors: %v", response.Errors)
	}
	var answer struct {
		LatestPrices []struct {
			MarketID    int
			Price       string
			BlockNumber int
		}
		PriceHistory []struct {
			BlockNumber int
			Price       string
		}
		ProtocolEvents []struct{ BlockNumber int }
		Market         struct {
			LatestPrice struct{ BlockNumber int }
			PoolState   struct{ BlockNumber int }
		}
	}
	if err := json.Unmarshal(response.Data, &answer); err != nil {
		t.Fatal(err)
	}
	if len(answer.LatestPrices) != 1 || answer.LatestPrices[0].BlockNumber != 3 || answer.LatestPrices[0].Price != "300" ||
		len(answer.PriceHistory) != 1 || answer.PriceHistory[0].BlockNumber != 3 || answer.PriceHistory[0].Price != "300" || len(answer.ProtocolEvents) != 0 ||
		answer.Market.LatestPrice.BlockNumber != 3 || answer.Market.PoolState.BlockNumber != 3 {
		t.Fatalf("retained query data: %s", response.Data)
	}
	// The real outbox publisher carries the decoder's envelopes through NATS
	// into the gateway feed; no price notification is manufactured by the test.
	nc, _ := subBroker(t)
	callbackErrors := make(chan error, 1)
	subscription, err := nc.Subscribe("perps.125.>", func(message *nats.Msg) {
		envelope, err := delivery.Decode(message.Data, store.Policy(125), message.Subject)
		if err != nil {
			select {
			case callbackErrors <- err:
			default:
			}
			return
		}
		feed.Deliver(message.Subject, envelope)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Unsubscribe()
	if err := nc.Flush(); err != nil {
		t.Fatal(err)
	}
	publisher, err := publish.New(db, nc, store.Policy(125), publish.Config{Owner: "prices", Batch: 100, MaxAttempts: 2,
		LeaseDuration: time.Second, PublishTimeout: 500 * time.Millisecond, RetryDelay: time.Second, IdleInterval: 5 * time.Millisecond, MaxHeadAge: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	publishCtx, stopPublisher := context.WithCancel(ctx)
	published := make(chan error, 1)
	go func() { published <- publisher.Run(publishCtx) }()
	defer func() {
		stopPublisher()
		if err := <-published; err != nil && !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
	}()
	assertUpdate := func(want int32) {
		t.Helper()
		for _, ch := range []<-chan *subPriceUpdate{all, one} {
			value := retentionReceive(t, ctx, ch)
			if value.BlockNumber != want || string(value.Price) != strconv.Itoa(int(want)*100) {
				t.Fatalf("subscription value %+v, want block %d", value, want)
			}
		}
		values := retentionReceive(t, ctx, latest)
		if len(values) != 1 || values[0].BlockNumber != want {
			t.Fatalf("latest subscription %+v, want block %d", values, want)
		}
	}
	assertUpdate(3)
	// Restart subscriptions with sparse history, then consume a fresh insert.
	stop()
	for range all {
	}
	for range one {
	}
	for range latest {
	}
	stop, all, one, latest = subscribe(ctx)
	write(4, now.Add(time.Second))
	assertUpdate(4)
	if _, err := retention.Pass(ctx, db, now.AddDate(0, 0, -7), "indexer", "writer"); err != nil {
		t.Fatal(err)
	}
	write(5, now.Add(2*time.Second))
	assertUpdate(5)
	history, err := resolver.PriceHistory(ctx, struct {
		MarketID int32
		Limit    *int32
		Offset   *int32
	}{MarketID: 0})
	if err != nil || len(history) != 3 || history[0].BlockNumber != 5 || history[1].BlockNumber != 4 || history[2].BlockNumber != 3 {
		t.Fatalf("history after new inserts: %+v %v", history, err)
	}
	select {
	case err := <-callbackErrors:
		t.Fatal(err)
	default:
	}
}

func TestPriceRetentionSchemaDescriptions(t *testing.T) {
	schema, err := NewSchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	result := schema.Exec(context.Background(), `{ __type(name:"Query") { fields { name description } } }`, "", nil)
	if len(result.Errors) != 0 {
		t.Fatal(result.Errors)
	}
	var answer struct {
		Type struct {
			Fields []struct {
				Name        string
				Description string
			}
		} `json:"__type"`
	}
	if err := json.Unmarshal(result.Data, &answer); err != nil {
		t.Fatal(err)
	}
	descriptions := map[string]string{}
	for _, field := range answer.Type.Fields {
		descriptions[field.Name] = field.Description
	}
	for _, name := range []string{"priceHistory", "protocolEvents"} {
		if !strings.Contains(descriptions[name], "PERPS_PRICE_RETENTION_DAYS") || !strings.Contains(descriptions[name], "seven days") {
			t.Fatalf("%s description: %s", name, descriptions[name])
		}
	}
	if !strings.Contains(descriptions["priceHistory"], "per UTC minute") || !strings.Contains(descriptions["protocolEvents"], "remaining events") {
		t.Fatalf("retention descriptions: %v", descriptions)
	}
}
